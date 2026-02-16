#!/usr/bin/env python3
# Parse existing customers.name into contact/phone/address (best-effort)
import re
import subprocess
import json

SCHEMA='p2rms15pepb5ciz'

def psql(query: str):
    cmd = ['docker','exec','-i','erp_postgres','psql','-U','nocodb','-d','nocodb','-At','-F','\t','-c',query]
    out = subprocess.check_output(cmd, text=True)
    return out

phone_re = re.compile(r'(1\d{10})')


def parse(raw: str):
    s = (raw or '').strip()
    phone = None
    m = phone_re.search(s)
    if m:
        phone = m.group(1)
    # contact: try patterns
    contact = None
    # look around phone
    if phone:
        before = s[:m.start()].strip()
        after = s[m.end():].strip()
        # common separators
        before = re.sub(r'[，,|:：\s]+$', '', before)
        # take last token of before as contact if short
        token = re.split(r'[，,|:：\s]+', before)
        token = [t for t in token if t]
        if token:
            cand = token[-1]
            if 1 <= len(cand) <= 6 and not re.search(r'\d', cand) and not any(k in cand for k in ['地址','收件','电话','手机','联系人','所在地区','详细']):
                contact = cand
        # address: remove obvious labels
        addr = s
        if phone:
            addr = addr.replace(phone, '')
        addr = re.sub(r'收件人\s*[:：]?','',addr)
        addr = re.sub(r'姓名\s*[:：]?','',addr)
        addr = re.sub(r'手机号码\s*[:：]?','',addr)
        addr = re.sub(r'联系电话\s*[:：]?','',addr)
        addr = re.sub(r'电话\s*[:：]?','',addr)
        addr = re.sub(r'收货地址\s*[:：]?','',addr)
        addr = re.sub(r'地址\s*[:：]?','',addr)
        addr = re.sub(r'所在地区\s*[:：]?','',addr)
        addr = re.sub(r'详细地址\s*[:：]?','',addr)
        addr = re.sub(r'[|，,]+',' ',addr)
        addr = re.sub(r'\s+',' ',addr).strip()
        address = addr
    else:
        address = s
    return contact, phone, address


def main():
    rows = psql(
        f"SELECT id, regexp_replace(regexp_replace(name, E'\\t', ' ', 'g'), E'\\n', ' ', 'g') "
        f"FROM {SCHEMA}.customers WHERE (phone IS NULL OR phone='') OR (address IS NULL OR address='') ORDER BY id;"
    )
    updates=[]
    for line in rows.splitlines():
        if not line.strip():
            continue
        parts = line.split('\t',1)
        if len(parts) != 2:
            continue
        cid, name = parts
        contact, phone, address = parse(name)
        updates.append((int(cid), contact, phone, address))

    # emit sql
    stmts=['BEGIN;']
    for cid, contact, phone, address in updates:
        def esc(x):
            if x is None:
                return 'NULL'
            return "'"+x.replace("'","''")+"'"
        stmts.append(
            f"UPDATE {SCHEMA}.customers SET contact={esc(contact)}, phone={esc(phone)}, address={esc(address)}, updated_at=now() WHERE id={cid};"
        )
    stmts.append('COMMIT;')
    sql='\n'.join(stmts)+'\n'
    open('/tmp/customer_parse.sql','w').write(sql)
    subprocess.run(['docker','exec','-i','erp_postgres','psql','-U','nocodb','-d','nocodb'], input=sql.encode('utf-8'), check=True)
    print('updated',len(updates))

if __name__=='__main__':
    main()
