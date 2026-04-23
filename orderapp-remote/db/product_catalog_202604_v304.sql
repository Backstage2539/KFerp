-- KFerp product catalog sync from "云南孟连棵凡-熟豆商用 v3.0.4" (2026-04).
-- Internal order pricing is stored as Yuan/lb and matched against total order weight in lb.
-- 454g package rows therefore use the published package price directly; 227g rows
-- are doubled to Yuan/lb; kg rows are converted with 1lb = 454g.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TEMP TABLE catalog_import_202604_v304 (
  name TEXT PRIMARY KEY,
  default_price NUMERIC NOT NULL
) ON COMMIT DROP;

INSERT INTO catalog_import_202604_v304 (name, default_price) VALUES
  ('曲奇拼配', 37.18714),
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
  ('白月光-瑰夏', 380),
  ('芸上莓梦', 164),
  ('晨曦-娜伊', 474),
  ('晚香玉', 140);

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
  min_qty_lb NUMERIC NOT NULL,
  max_qty_lb NUMERIC,
  price_per_lb NUMERIC NOT NULL
) ON COMMIT DROP;

INSERT INTO catalog_tiers_202604_v304 (name, min_qty_lb, max_qty_lb, price_per_lb) VALUES
  -- Factory kg pricing: published Yuan/kg converted to Yuan/lb; kg thresholds converted to lb.
  ('曲奇拼配', 52.86343612334802, 107.9295154185022, 37.18714),
  ('曲奇拼配', 110.13215859030837, 218.06167400881057, 35.41654),
  ('曲奇拼配', 220.26431718061673, 438.3259911894273, 33.03304),

  -- Discount batch prices.
  ('浣纱果园', 2, 13, 99),
  ('浣纱果园', 14, NULL, 87),
  ('肯尼亚TOP AA', 2, 13, 117),
  ('肯尼亚TOP AA', 14, NULL, 103),

  -- 454g package rows. Tier boundaries follow the latest manually entered 橘皮乌龙 pattern.
  ('金色山脉', 2, 13, 61),
  ('金色山脉', 14, 23, 55),
  ('金色山脉', 24, 48, 49),
  ('金色山脉', 49, NULL, 46),
  ('酒心巧克力', 2, 13, 65),
  ('酒心巧克力', 14, 23, 59),
  ('酒心巧克力', 24, 48, 53),
  ('酒心巧克力', 49, NULL, 48),
  ('菠萝意式2.0', 2, 13, 106),
  ('菠萝意式2.0', 14, 23, 92),
  ('菠萝意式2.0', 24, 48, 81),
  ('菠萝意式2.0', 49, NULL, 74),
  ('橘皮乌龙', 2, 13, 61),
  ('橘皮乌龙', 14, 23, 55),
  ('橘皮乌龙', 24, 48, 49),
  ('橘皮乌龙', 49, NULL, 46),
  ('芒霜2.0', 2, 13, 116),
  ('芒霜2.0', 14, 23, 101),
  ('芒霜2.0', 24, 48, 89),
  ('芒霜2.0', 49, NULL, 82),
  ('小菠萝2.0', 2, 13, 106),
  ('小菠萝2.0', 14, 23, 92),
  ('小菠萝2.0', 24, 48, 81),
  ('小菠萝2.0', 49, NULL, 74),
  ('萨奇姆', 2, 13, 100),
  ('萨奇姆', 14, 48, 87),
  ('萨奇姆', 49, NULL, 70),
  ('曜石2.0', 2, 13, 63),
  ('曜石2.0', 14, 23, 56),
  ('曜石2.0', 24, 48, 50),
  ('曜石2.0', 49, NULL, 47),
  ('红岩2.0', 2, 13, 63),
  ('红岩2.0', 14, 23, 56),
  ('红岩2.0', 24, 48, 50),
  ('红岩2.0', 49, NULL, 47),
  ('初晓', 2, 13, 73),
  ('初晓', 14, 23, 66),
  ('初晓', 24, 48, 59),
  ('初晓', 49, NULL, 55),
  ('松饼', 2, 13, 63),
  ('松饼', 14, 23, 57),
  ('松饼', 24, 48, 51),
  ('松饼', 49, NULL, 48),
  ('榛巧', 2, 13, 63),
  ('榛巧', 14, 23, 57),
  ('榛巧', 24, 48, 51),
  ('榛巧', 49, NULL, 48),
  ('果语花香', 2, 13, 69),
  ('果语花香', 14, 23, 62),
  ('果语花香', 24, 48, 56),
  ('果语花香', 49, NULL, 52),
  ('耶加雪菲G2', 2, 13, 86),
  ('耶加雪菲G2', 14, 23, 75),
  ('耶加雪菲G2', 24, 48, 65),
  ('耶加雪菲G2', 49, NULL, 56),
  ('Uraga乌拉嘎', 2, 13, 119),
  ('Uraga乌拉嘎', 14, 23, 104),
  ('Uraga乌拉嘎', 24, 48, 91),
  ('Uraga乌拉嘎', 49, NULL, 84),
  ('森林瑰夏', 2, 13, 130),
  ('森林瑰夏', 14, 23, 113),
  ('森林瑰夏', 24, 48, 99),
  ('森林瑰夏', 49, NULL, 91),
  ('Nenka嫩咖', 2, 13, 127),
  ('Nenka嫩咖', 14, 23, 111),
  ('Nenka嫩咖', 24, 48, 98),
  ('Nenka嫩咖', 49, NULL, 90),
  ('曼特宁', 2, 13, 79),
  ('曼特宁', 14, 23, 68),
  ('曼特宁', 24, 48, 60),
  ('曼特宁', 49, NULL, 55),

  -- 227g package rows: published Yuan/227g package converted to Yuan/lb.
  ('白月光-瑰夏', 1, 3, 380),
  ('白月光-瑰夏', 3.5, NULL, 330),
  ('芸上莓梦', 1, 3, 164),
  ('芸上莓梦', 3.5, NULL, 144),
  ('晨曦-娜伊', 1, 3, 474),
  ('晨曦-娜伊', 3.5, NULL, 368),
  ('晚香玉', 1, 3, 140),
  ('晚香玉', 3.5, NULL, 122);

INSERT INTO p2rms15pepb5ciz.product_price_tiers (product_id, min_qty_lb, max_qty_lb, price_per_lb, active)
SELECT p.id, t.min_qty_lb, t.max_qty_lb, t.price_per_lb, TRUE
FROM catalog_tiers_202604_v304 t
JOIN p2rms15pepb5ciz.products p ON p.name = t.name
ORDER BY p.name, t.min_qty_lb;

COMMIT;
