-- KFerp retail product price sync from "云南孟连棵凡-熟豆零售 v3.0.2" (2026-04).
-- Retail prices are stored as Yuan per exact package spec. Order entry uses an
-- exact configured spec price first, then falls back to 227g conversion.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE p2rms15pepb5ciz.products ADD COLUMN IF NOT EXISTS retail_price_100g NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE p2rms15pepb5ciz.products ADD COLUMN IF NOT EXISTS retail_price_200g NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE p2rms15pepb5ciz.products ADD COLUMN IF NOT EXISTS retail_price_227g NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE p2rms15pepb5ciz.products ADD COLUMN IF NOT EXISTS retail_price_250g NUMERIC(12,2) NOT NULL DEFAULT 0;

CREATE TEMP TABLE retail_import_202604_v302 (
  name TEXT PRIMARY KEY,
  price_100g NUMERIC NOT NULL DEFAULT 0,
  price_200g NUMERIC NOT NULL DEFAULT 0,
  price_227g NUMERIC NOT NULL DEFAULT 0,
  price_250g NUMERIC NOT NULL DEFAULT 0
) ON COMMIT DROP;

INSERT INTO retail_import_202604_v302 (name, price_100g, price_200g, price_227g, price_250g) VALUES
  ('金色山脉', 0, 0, 50, 55),
  ('酒心巧克力', 0, 0, 53, 59),
  ('菠萝意式2.0', 0, 0, 73, 80),
  ('橘皮乌龙', 0, 0, 50, 55),
  ('芒霜2.0', 0, 0, 80, 88),
  ('小菠萝2.0', 0, 0, 73, 80),
  ('萨奇姆', 0, 0, 69, 76),
  ('曜石2.0', 0, 0, 51, 56),
  ('红岩2.0', 0, 0, 47, 51),
  ('初晓', 0, 0, 59, 65),
  ('松饼', 0, 0, 51, 57),
  ('榛巧', 0, 0, 51, 57),
  ('果语花香', 0, 0, 56, 62),
  ('耶加雪菲G2', 0, 0, 60, 66),
  ('Uraga乌拉嘎', 0, 0, 82, 90),
  ('浣纱果园', 0, 0, 77, 84),
  ('肯尼亚TOP AA', 0, 0, 91, 100),
  ('森林瑰夏', 0, 0, 89, 98),
  ('Nenka嫩咖', 0, 0, 88, 97),
  ('曼特宁', 0, 0, 55, 60),
  ('白月光-瑰夏', 115, 229, 0, 0),
  ('芸上莓梦', 50, 100, 0, 0),
  ('晨曦-娜伊', 143, 286, 0, 0),
  ('晚香玉', 42, 85, 0, 0);

UPDATE p2rms15pepb5ciz.products p
SET retail_price_100g = r.price_100g,
    retail_price_200g = r.price_200g,
    retail_price_227g = r.price_227g,
    retail_price_250g = r.price_250g
FROM retail_import_202604_v302 r
WHERE p.name = r.name;

COMMIT;
