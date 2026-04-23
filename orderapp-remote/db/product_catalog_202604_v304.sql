-- KFerp product catalog sync from "云南孟连棵凡-熟豆商用 v3.0.4" (2026-04).
-- Wholesale tiers are stored by package spec (g), package-count range, and Yuan/package.
-- Legacy lb columns are still populated for older code paths, but the current app
-- matches by spec_g + min_qty_units/max_qty_units and charges price_per_unit * units.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE p2rms15pepb5ciz.product_price_tiers ADD COLUMN IF NOT EXISTS spec_g BIGINT NOT NULL DEFAULT 454;
ALTER TABLE p2rms15pepb5ciz.product_price_tiers ADD COLUMN IF NOT EXISTS min_qty_units NUMERIC NOT NULL DEFAULT 0;
ALTER TABLE p2rms15pepb5ciz.product_price_tiers ADD COLUMN IF NOT EXISTS max_qty_units NUMERIC;
ALTER TABLE p2rms15pepb5ciz.product_price_tiers ADD COLUMN IF NOT EXISTS price_per_unit NUMERIC(12,2) NOT NULL DEFAULT 0;

CREATE TEMP TABLE catalog_import_202604_v304 (
  name TEXT PRIMARY KEY,
  default_price NUMERIC NOT NULL
) ON COMMIT DROP;

INSERT INTO catalog_import_202604_v304 (name, default_price) VALUES
  ('曲奇拼配', 81.91),
  ('浣纱果园', 99),
  ('肯尼亚TOP AA', 117),
  ('金色山脉', 61),
  ('酒心巧克力', 65),
  ('菠萝意式2.0', 106),
  ('橘皮乌龙', 61),
  ('芒霜2.0', 116),
  ('小菠萝2.0', 106),
  ('萨奇姆', 100),
  ('曜石2.0', 63),
  ('红岩2.0', 63),
  ('初晓', 73),
  ('松饼', 63),
  ('榛巧', 63),
  ('果语花香', 69),
  ('耶加雪菲G2', 86),
  ('Uraga乌拉嘎', 119),
  ('森林瑰夏', 130),
  ('Nenka嫩咖', 127),
  ('曼特宁', 79),
  ('白月光-瑰夏', 190),
  ('芸上莓梦', 82),
  ('晨曦-娜伊', 237),
  ('晚香玉', 70);

UPDATE p2rms15pepb5ciz.products
SET active = FALSE
WHERE active = TRUE
  AND name NOT IN (SELECT name FROM catalog_import_202604_v304);

INSERT INTO p2rms15pepb5ciz.products (name, default_price, active)
SELECT name, default_price, TRUE
FROM catalog_import_202604_v304
ON CONFLICT (name) DO UPDATE
SET default_price = EXCLUDED.default_price,
    active = TRUE;

UPDATE p2rms15pepb5ciz.product_price_tiers t
SET active = FALSE
FROM p2rms15pepb5ciz.products p, catalog_import_202604_v304 c
WHERE t.product_id = p.id
  AND p.name = c.name
  AND t.active = TRUE;

CREATE TEMP TABLE catalog_tiers_202604_v304 (
  name TEXT NOT NULL,
  spec_g BIGINT NOT NULL,
  min_qty_units NUMERIC NOT NULL,
  max_qty_units NUMERIC,
  price_per_unit NUMERIC NOT NULL
) ON COMMIT DROP;

INSERT INTO catalog_tiers_202604_v304 (name, spec_g, min_qty_units, max_qty_units, price_per_unit) VALUES
  -- Factory kg pricing: Yuan/kg, represented as 1000g packages.
  ('曲奇拼配', 1000, 24, 49, 81.91),
  ('曲奇拼配', 1000, 50, 99, 78.01),
  ('曲奇拼配', 1000, 100, 199, 72.76),

  -- Discount batch prices, 454g packages.
  ('浣纱果园', 454, 2, 13, 99),
  ('浣纱果园', 454, 14, NULL, 87),
  ('肯尼亚TOP AA', 454, 2, 13, 117),
  ('肯尼亚TOP AA', 454, 14, NULL, 103),

  -- 454g package rows.
  ('金色山脉', 454, 2, 13, 61),
  ('金色山脉', 454, 14, 23, 55),
  ('金色山脉', 454, 24, 48, 49),
  ('金色山脉', 454, 49, NULL, 46),
  ('酒心巧克力', 454, 2, 13, 65),
  ('酒心巧克力', 454, 14, 23, 59),
  ('酒心巧克力', 454, 24, 48, 53),
  ('酒心巧克力', 454, 49, NULL, 48),
  ('菠萝意式2.0', 454, 2, 13, 106),
  ('菠萝意式2.0', 454, 14, 23, 92),
  ('菠萝意式2.0', 454, 24, 48, 81),
  ('菠萝意式2.0', 454, 49, NULL, 74),
  ('橘皮乌龙', 454, 2, 13, 61),
  ('橘皮乌龙', 454, 14, 23, 55),
  ('橘皮乌龙', 454, 24, 48, 49),
  ('橘皮乌龙', 454, 49, NULL, 46),
  ('芒霜2.0', 454, 2, 13, 116),
  ('芒霜2.0', 454, 14, 23, 101),
  ('芒霜2.0', 454, 24, 48, 89),
  ('芒霜2.0', 454, 49, NULL, 82),
  ('小菠萝2.0', 454, 2, 13, 106),
  ('小菠萝2.0', 454, 14, 23, 92),
  ('小菠萝2.0', 454, 24, 48, 81),
  ('小菠萝2.0', 454, 49, NULL, 74),
  ('萨奇姆', 454, 2, 13, 100),
  ('萨奇姆', 454, 14, 48, 87),
  ('萨奇姆', 454, 49, NULL, 70),
  ('曜石2.0', 454, 2, 13, 63),
  ('曜石2.0', 454, 14, 23, 56),
  ('曜石2.0', 454, 24, 48, 50),
  ('曜石2.0', 454, 49, NULL, 47),
  ('红岩2.0', 454, 2, 13, 63),
  ('红岩2.0', 454, 14, 23, 56),
  ('红岩2.0', 454, 24, 48, 50),
  ('红岩2.0', 454, 49, NULL, 47),
  ('初晓', 454, 2, 13, 73),
  ('初晓', 454, 14, 23, 66),
  ('初晓', 454, 24, 48, 59),
  ('初晓', 454, 49, NULL, 55),
  ('松饼', 454, 2, 13, 63),
  ('松饼', 454, 14, 23, 57),
  ('松饼', 454, 24, 48, 51),
  ('松饼', 454, 49, NULL, 48),
  ('榛巧', 454, 2, 13, 63),
  ('榛巧', 454, 14, 23, 57),
  ('榛巧', 454, 24, 48, 51),
  ('榛巧', 454, 49, NULL, 48),
  ('果语花香', 454, 2, 13, 69),
  ('果语花香', 454, 14, 23, 62),
  ('果语花香', 454, 24, 48, 56),
  ('果语花香', 454, 49, NULL, 52),
  ('耶加雪菲G2', 454, 2, 13, 86),
  ('耶加雪菲G2', 454, 14, 23, 75),
  ('耶加雪菲G2', 454, 24, 48, 65),
  ('耶加雪菲G2', 454, 49, NULL, 56),
  ('Uraga乌拉嘎', 454, 2, 13, 119),
  ('Uraga乌拉嘎', 454, 14, 23, 104),
  ('Uraga乌拉嘎', 454, 24, 48, 91),
  ('Uraga乌拉嘎', 454, 49, NULL, 84),
  ('森林瑰夏', 454, 2, 13, 130),
  ('森林瑰夏', 454, 14, 23, 113),
  ('森林瑰夏', 454, 24, 48, 99),
  ('森林瑰夏', 454, 49, NULL, 91),
  ('Nenka嫩咖', 454, 2, 13, 127),
  ('Nenka嫩咖', 454, 14, 23, 111),
  ('Nenka嫩咖', 454, 24, 48, 98),
  ('Nenka嫩咖', 454, 49, NULL, 90),
  ('曼特宁', 454, 2, 13, 79),
  ('曼特宁', 454, 14, 23, 68),
  ('曼特宁', 454, 24, 48, 60),
  ('曼特宁', 454, 49, NULL, 55),

  -- 227g package rows.
  ('白月光-瑰夏', 227, 2, 6, 190),
  ('白月光-瑰夏', 227, 7, NULL, 165),
  ('芸上莓梦', 227, 2, 6, 82),
  ('芸上莓梦', 227, 7, NULL, 72),
  ('晨曦-娜伊', 227, 2, 6, 237),
  ('晨曦-娜伊', 227, 7, NULL, 184),
  ('晚香玉', 227, 2, 6, 70),
  ('晚香玉', 227, 7, NULL, 61);

INSERT INTO p2rms15pepb5ciz.product_price_tiers
  (product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
SELECT p.id,
       t.spec_g,
       t.min_qty_units,
       t.max_qty_units,
       t.price_per_unit,
       t.min_qty_units * t.spec_g / 454.0,
       CASE WHEN t.max_qty_units IS NULL THEN NULL ELSE t.max_qty_units * t.spec_g / 454.0 END,
       t.price_per_unit * 454.0 / t.spec_g,
       TRUE
FROM catalog_tiers_202604_v304 t
JOIN p2rms15pepb5ciz.products p ON p.name = t.name
ORDER BY p.name, t.spec_g, t.min_qty_units;

COMMIT;
