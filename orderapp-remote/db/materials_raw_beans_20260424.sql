-- KFerp raw bean material import from Van screenshot (2026-04-24).
-- One row in the screenshot is partially clipped; "白月光" below is inferred from
-- the current product catalog and the visible trailing "月".
-- This script is idempotent by material name and preserves existing inventory.

CREATE TEMP TABLE material_import_20260424 (
  name TEXT NOT NULL,
  purchase_price NUMERIC(12,2) NOT NULL
);

INSERT INTO material_import_20260424 (name, purchase_price) VALUES
  ('卡蒂姆水洗', 54.000),
  ('白月光', 54.000),
  ('卡蒂姆慢速日晒', 105.000),
  ('卡蒂姆红酒日晒', 67.000),
  ('黄波旁水洗', 72.000),
  ('黄波旁日晒', 95.000),
  ('曼特宁DP', 69.000),
  ('哥伦比亚', 82.000),
  ('耶加雪菲G1', 110.000),
  ('耶加雪菲G2', 76.000),
  ('肯尼亚TOPAA', 120.000),
  ('埃塞花魁日晒', 104.000),
  ('巴西', 78.000),
  ('浣纱', 100.000),
  ('萨奇姆', 90.000),
  ('ALO 688', 288.000),
  ('秘鲁瑰夏', 450.000),
  ('精品卡蒂姆水洗', 62.000),
  ('印尼铁皮卡', 128.000),
  ('哥斯达黎加', 82.000),
  ('乌干达（罗）', 45.000),
  ('西达摩G4', 68.000),
  ('朵望丘', 0.000),
  ('如木达摩', 152.000),
  ('乌拉嘎', 108.000),
  ('森林瑰夏水洗', 118.000),
  ('森林瑰夏日晒', 0.000),
  ('嫩咖', 116.000),
  ('洪都拉斯瑰夏', 450.000);

UPDATE p2rms15pepb5ciz.materials m
SET kind = 'bean',
    unit = 'g',
    purchase_price = i.purchase_price,
    updated_at = now()
FROM material_import_20260424 i
WHERE m.name = i.name;

INSERT INTO p2rms15pepb5ciz.materials (
  code, name, kind, unit, purchase_price, sale_price,
  onhand_g, onhand_units, min_level_g, min_level_units, updated_at
)
SELECT
  'bean-' || substr(md5(i.name), 1, 10),
  i.name,
  'bean',
  'g',
  i.purchase_price,
  0,
  0,
  0,
  0,
  0,
  now()
FROM material_import_20260424 i
WHERE NOT EXISTS (
  SELECT 1 FROM p2rms15pepb5ciz.materials m WHERE m.name = i.name
);
