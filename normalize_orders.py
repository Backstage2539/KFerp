import re, subprocess, csv, sys, datetime

SCHEMA = "p2rms15pepb5ciz"
SRC_TABLE = "orders_2026_01_02_csv"

base_date = datetime.date(1899, 12, 30)


def to_date(s):
    if s is None:
        return None
    s = str(s).strip()
    if not s:
        return None
    if re.match(r"^(\d{4})-(\d{2})-(\d{2})$", s):
        return s
    if re.match(r"^\d+(\.\d+)?$", s):
        try:
            n = float(s)
            if 20000 < n < 80000:
                d = base_date + datetime.timedelta(days=int(n))
                return d.isoformat()
        except Exception:
            pass
    return None


def to_number(s):
    if s is None:
        return None
    s = str(s).strip()
    if not s:
        return None
    m = re.search(r"-?\d+(?:\.\d+)?", s)
    if not m:
        return None
    try:
        return float(m.group(0))
    except Exception:
        return None


unit_norm = {
    "公斤": "kg",
    "千克": "kg",
    "kg": "kg",
    "KG": "kg",
    "Kg": "kg",
    "克": "g",
    "g": "g",
    "G": "g",
    "磅": "lb",
    "盒": "box",
    "袋": "bag",
    "包": "pack",
    "瓶": "bottle",
}


def parse_line(line: str):
    raw = line.strip()
    if not raw:
        return None

    # remove bullets/numbering
    raw = re.sub(r"^[\s\-\*\d️⃣①②③④⑤⑥⑦⑧⑨\.]+", "", raw)

    # patterns like "227g*4袋" or "454g×3"
    m = re.search(
        r"(?P<spec>\d+(?:\.\d+)?)\s*(?P<spec_unit>kg|KG|g|G|克|公斤)\s*[xX\*×]\s*(?P<qty>\d+(?:\.\d+)?)\s*(?P<qty_unit>袋|包|盒|瓶)?",
        raw,
    )
    if m:
        spec = float(m.group("spec"))
        su = unit_norm.get(m.group("spec_unit"), m.group("spec_unit"))
        qty = float(m.group("qty"))
        qu = unit_norm.get(m.group("qty_unit") or "", m.group("qty_unit") or "")
        name = raw[: m.start()].strip() or raw
        return {
            "name": name,
            "qty": qty,
            "unit": qu or "count",
            "spec": f"{spec:g}{su}",
            "raw": line.strip(),
        }

    # patterns like "初晓 96磅" or "挂耳 20盒"
    m = re.search(
        r"(?P<name>.+?)\s*(?P<qty>\d+(?:\.\d+)?)\s*(?P<unit>kg|KG|g|G|克|公斤|磅|盒|袋|包|瓶)\b",
        raw,
    )
    if m:
        name = m.group("name").strip(" ：:，,\t")
        qty = float(m.group("qty"))
        unit = unit_norm.get(m.group("unit"), m.group("unit"))
        return {"name": name, "qty": qty, "unit": unit, "spec": None, "raw": line.strip()}

    return {"name": None, "qty": None, "unit": None, "spec": None, "raw": line.strip()}


def psql(query: str):
    cmd = [
        "docker",
        "exec",
        "-i",
        "erp_postgres",
        "psql",
        "-U",
        "nocodb",
        "-d",
        "nocodb",
        "-v",
        "ON_ERROR_STOP=1",
        "-q",
        "-t",
        "-A",
        "-F",
        "\t",
        "-P",
        "footer=off",
        "-c",
        query,
    ]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode != 0:
        sys.stderr.write(res.stderr)
        raise SystemExit(res.returncode)
    return res.stdout


def main():
    # Create tables
    psql(
        f"""
BEGIN;
CREATE TABLE IF NOT EXISTS {SCHEMA}.orders (
  id bigserial PRIMARY KEY,
  src_row_id integer,
  sheet text,
  legacy_seq text,
  ship_status text,
  order_date date,
  customer text,
  notes text,
  grind text,
  roast_level text,
  pay_status text,
  total_amount numeric,
  shipping_amount numeric,
  paid_date date,
  source text,
  express_fee text,
  order_type text,
  tracking_no text,
  shipped_date date,
  term text,
  items_raw text,
  order_no text
);

CREATE TABLE IF NOT EXISTS {SCHEMA}.order_items (
  id bigserial PRIMARY KEY,
  order_id bigint REFERENCES {SCHEMA}.orders(id) ON DELETE CASCADE,
  line_no integer,
  item_name text,
  qty numeric,
  unit text,
  spec text,
  raw_line text
);
COMMIT;
"""
    )

    # Use CSV output to safely handle embedded newlines in text fields
    copy_query = (
        f"\\copy (SELECT id,_sheet,\"序号\",\"发货状态\",\"订单日期\",\"客户\",\"备注\",\"磨粉\",\"品种\",\"烘焙程度\",\"付款状态\",\"货款_运费_元_\",\"运费_元_\",\"收款时间\",\"订单来源\",\"快递费\",\"订单类型\",\"单号\",\"发货日期\",\"账期\" FROM {SCHEMA}.{SRC_TABLE} ORDER BY id) TO STDOUT WITH CSV"
    )
    raw_csv = psql(copy_query)

    import io

    rows = []
    r = csv.reader(io.StringIO(raw_csv))
    for rec in r:
        if not rec or rec[0] == "id":
            continue
        # expect 20 columns
        if len(rec) < 20:
            # pad
            rec = rec + [""] * (20 - len(rec))
        rows.append(rec)

    out_orders = "/tmp/eve/_orders_norm.tsv"
    out_items = "/tmp/eve/_order_items.tsv"

    with open(out_orders, "w", encoding="utf-8") as f:
        w = csv.writer(f, delimiter="\t", lineterminator="\n")
        for p in rows:
            src_id = int(p[0])
            sheet, legacy_seq, ship_status = p[1], p[2], p[3]
            order_date = to_date(p[4])
            customer, notes, grind = p[5], p[6], p[7]
            items_raw, roast_level, pay_status = p[8], p[9], p[10]
            total_amt, ship_amt = to_number(p[11]), to_number(p[12])
            paid_date = to_date(p[13])
            source, express_fee, order_type = p[14], p[15], p[16]
            tracking_no = p[17]
            shipped_date = to_date(p[18])
            term = p[19]
            w.writerow(
                [
                    src_id,
                    sheet or "",
                    legacy_seq or "",
                    ship_status or "",
                    order_date or "",
                    customer or "",
                    notes or "",
                    grind or "",
                    roast_level or "",
                    pay_status or "",
                    "" if total_amt is None else total_amt,
                    "" if ship_amt is None else ship_amt,
                    paid_date or "",
                    source or "",
                    express_fee or "",
                    order_type or "",
                    tracking_no or "",
                    shipped_date or "",
                    term or "",
                    items_raw or "",
                ]
            )

    # idempotent refresh for imported rows
    psql(
        f"BEGIN; DELETE FROM {SCHEMA}.order_items WHERE order_id IN (SELECT id FROM {SCHEMA}.orders WHERE src_row_id IS NOT NULL); DELETE FROM {SCHEMA}.orders WHERE src_row_id IS NOT NULL; COMMIT;"
    )

    # copy files into postgres container because we run psql inside container
    subprocess.run(["docker", "exec", "-i", "erp_postgres", "bash", "-lc", "mkdir -p /tmp/eve"], check=True)
    subprocess.run(["docker", "cp", out_orders, "erp_postgres:/tmp/eve/_orders_norm.tsv"], check=True)

    psql(
        f"\\copy {SCHEMA}.orders (src_row_id,sheet,legacy_seq,ship_status,order_date,customer,notes,grind,roast_level,pay_status,total_amount,shipping_amount,paid_date,source,express_fee,order_type,tracking_no,shipped_date,term,items_raw) FROM '/tmp/eve/_orders_norm.tsv' WITH (FORMAT csv, DELIMITER E'\\t', HEADER false);"
    )

    # order_no SO-YYYYMMDD-0001 by date
    psql(
        f"""
WITH ranked AS (
  SELECT id, order_date,
         row_number() OVER (PARTITION BY order_date ORDER BY id) AS rn
  FROM {SCHEMA}.orders
)
UPDATE {SCHEMA}.orders o
SET order_no = CASE
  WHEN r.order_date IS NULL THEN NULL
  ELSE 'SO-' || to_char(r.order_date,'YYYYMMDD') || '-' || lpad(r.rn::text, 4, '0')
END
FROM ranked r
WHERE o.id=r.id;
"""
    )

    map_txt = psql(f"SELECT src_row_id||'\t'||id FROM {SCHEMA}.orders WHERE src_row_id IS NOT NULL;")
    map_row = {}
    for ln in map_txt.splitlines():
        a, b = ln.split("\t")
        map_row[int(a)] = int(b)

    with open(out_items, "w", encoding="utf-8") as f:
        w = csv.writer(f, delimiter="\t", lineterminator="\n")
        for p in rows:
            src_id = int(p[0])
            order_id = map_row.get(src_id)
            if not order_id:
                continue
            items_raw = p[8] or ""
            lines = []
            for seg in re.split(r"\n+", items_raw):
                seg = seg.strip()
                if not seg:
                    continue
                for s2 in re.split(r"[；;]+", seg):
                    s2 = s2.strip()
                    if s2:
                        lines.append(s2)
            if not lines and items_raw.strip():
                lines = [items_raw.strip()]

            line_no = 0
            for ln in lines:
                line_no += 1
                parsed = parse_line(ln)
                if not parsed:
                    continue
                w.writerow(
                    [
                        order_id,
                        line_no,
                        parsed.get("name") or "",
                        "" if parsed.get("qty") is None else parsed.get("qty"),
                        parsed.get("unit") or "",
                        parsed.get("spec") or "",
                        parsed.get("raw") or "",
                    ]
                )

    psql(f"TRUNCATE {SCHEMA}.order_items RESTART IDENTITY;")
    subprocess.run(["docker", "cp", out_items, "erp_postgres:/tmp/eve/_order_items.tsv"], check=True)
    psql(
        f"\\copy {SCHEMA}.order_items (order_id,line_no,item_name,qty,unit,spec,raw_line) FROM '/tmp/eve/_order_items.tsv' WITH (FORMAT csv, DELIMITER E'\\t', HEADER false);"
    )

    print("orders", psql(f"SELECT count(*) FROM {SCHEMA}.orders;").strip())
    print("items", psql(f"SELECT count(*) FROM {SCHEMA}.order_items;").strip())
    print(
        psql(
            f"SELECT order_no, order_date, customer FROM {SCHEMA}.orders WHERE order_no IS NOT NULL ORDER BY id DESC LIMIT 3;"
        )
    )


if __name__ == "__main__":
    main()
