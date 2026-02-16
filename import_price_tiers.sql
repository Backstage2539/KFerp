-- products: 25
BEGIN;
SET LOCAL lock_timeout = '5s';
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('ALO688日晒', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('Nenka嫩咖', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('乌拉嘎', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('初晓', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('如目达摩', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('孟连浅烘', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('小菠萝', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('曜石2.0', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('曼特宁', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('松饼', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('森林瑰夏日晒', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('森林瑰夏水洗', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('浣纱果园', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('白月光', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('红岩2.0', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('红酒日晒', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('耶加雪菲G1', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('耶加雪菲G1（2024）', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('耶加雪菲G2（新产季）', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('肯尼亚TOPAA', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('芒霜', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('莓屋', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('菠萝意式', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('酒心巧克力', 0, TRUE) ON CONFLICT (name) DO NOTHING;
INSERT INTO p2rms15pepb5ciz.products(name, default_price, active) VALUES ('金色山脉', 0, TRUE) ON CONFLICT (name) DO NOTHING;
-- refresh tiers for imported products
DELETE FROM p2rms15pepb5ciz.product_price_tiers t USING p2rms15pepb5ciz.products p WHERE t.product_id=p.id AND p.name IN (
  'ALO688日晒',
  'Nenka嫩咖',
  '乌拉嘎',
  '初晓',
  '如目达摩',
  '孟连浅烘',
  '小菠萝',
  '曜石2.0',
  '曼特宁',
  '松饼',
  '森林瑰夏日晒',
  '森林瑰夏水洗',
  '浣纱果园',
  '白月光',
  '红岩2.0',
  '红酒日晒',
  '耶加雪菲G1',
  '耶加雪菲G1（2024）',
  '耶加雪菲G2（新产季）',
  '肯尼亚TOPAA',
  '芒霜',
  '莓屋',
  '菠萝意式',
  '酒心巧克力',
  '金色山脉'
);
-- ALO688日晒
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,6.0,163.00049052171053,TRUE FROM p2rms15pepb5ciz.products WHERE name='ALO688日晒';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,7.0,NULL,142.91523333092107,TRUE FROM p2rms15pepb5ciz.products WHERE name='ALO688日晒';
-- Nenka嫩咖
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,132.43119209605265,TRUE FROM p2rms15pepb5ciz.products WHERE name='Nenka嫩咖';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,115.84119876710525,TRUE FROM p2rms15pepb5ciz.products WHERE name='Nenka嫩咖';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,102.784917425,TRUE FROM p2rms15pepb5ciz.products WHERE name='Nenka嫩咖';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,94.65658655000001,TRUE FROM p2rms15pepb5ciz.products WHERE name='Nenka嫩咖';
-- 乌拉嘎
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,124.12562051710526,TRUE FROM p2rms15pepb5ciz.products WHERE name='乌拉嘎';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,108.63239560921052,TRUE FROM p2rms15pepb5ciz.products WHERE name='乌拉嘎';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,96.30729321447369,TRUE FROM p2rms15pepb5ciz.products WHERE name='乌拉嘎';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,88.72734655000001,TRUE FROM p2rms15pepb5ciz.products WHERE name='乌拉嘎';
-- 初晓
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,77.99999709605264,TRUE FROM p2rms15pepb5ciz.products WHERE name='初晓';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,70.56945376710527,TRUE FROM p2rms15pepb5ciz.products WHERE name='初晓';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,63.81576768815789,TRUE FROM p2rms15pepb5ciz.products WHERE name='初晓';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,60.21160655,TRUE FROM p2rms15pepb5ciz.products WHERE name='初晓';
-- 如目达摩
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,6.0,92.4031321006579,TRUE FROM p2rms15pepb5ciz.products WHERE name='如目达摩';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,7.0,NULL,81.64040648881577,TRUE FROM p2rms15pepb5ciz.products WHERE name='如目达摩';
-- 孟连浅烘
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,66.06094703026316,TRUE FROM p2rms15pepb5ciz.products WHERE name='孟连浅烘';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,59.87636488552633,TRUE FROM p2rms15pepb5ciz.products WHERE name='孟连浅烘';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,53.95331959605263,TRUE FROM p2rms15pepb5ciz.products WHERE name='孟连浅烘';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,50.972139049999996,TRUE FROM p2rms15pepb5ciz.products WHERE name='孟连浅烘';
-- 小菠萝
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,100.2471022276316,TRUE FROM p2rms15pepb5ciz.products WHERE name='小菠萝';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,NULL,87.90708653026316,TRUE FROM p2rms15pepb5ciz.products WHERE name='小菠萝';
-- 曜石2.0
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,71.72094113552632,TRUE FROM p2rms15pepb5ciz.products WHERE name='曜石2.0';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,64.94568109605264,TRUE FROM p2rms15pepb5ciz.products WHERE name='曜石2.0';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,58.62885054342105,TRUE FROM p2rms15pepb5ciz.products WHERE name='曜石2.0';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,55.35233105,TRUE FROM p2rms15pepb5ciz.products WHERE name='曜石2.0';
-- 曼特宁
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,83.63595906973686,TRUE FROM p2rms15pepb5ciz.products WHERE name='曼特宁';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,73.48948021447369,TRUE FROM p2rms15pepb5ciz.products WHERE name='曼特宁';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,64.7288751881579,TRUE FROM p2rms15pepb5ciz.products WHERE name='曼特宁';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,59.822301550000006,TRUE FROM p2rms15pepb5ciz.products WHERE name='曼特宁';
-- 松饼
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,72.42844039868423,TRUE FROM p2rms15pepb5ciz.products WHERE name='松饼';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,65.57934562236841,TRUE FROM p2rms15pepb5ciz.products WHERE name='松饼';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,59.213291911842106,TRUE FROM p2rms15pepb5ciz.products WHERE name='松饼';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,55.89985504999999,TRUE FROM p2rms15pepb5ciz.products WHERE name='松饼';
-- 森林瑰夏日晒
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,141.7749601223684,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏日晒';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,123.95110231973683,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏日晒';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,110.0722446618421,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏日晒';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,101.32698155000001,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏日晒';
-- 森林瑰夏水洗
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,134.50758499078947,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏水洗';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,117.64339955657896,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏水洗';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,104.40432347763158,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏水洗';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,96.13889655,TRUE FROM p2rms15pepb5ciz.products WHERE name='森林瑰夏水洗';
-- 浣纱果园
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,115.8200489381579,TRUE FROM p2rms15pepb5ciz.products WHERE name='浣纱果园';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,101.4235924513158,TRUE FROM p2rms15pepb5ciz.products WHERE name='浣纱果园';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,89.82966900394737,TRUE FROM p2rms15pepb5ciz.products WHERE name='浣纱果园';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,82.79810655,TRUE FROM p2rms15pepb5ciz.products WHERE name='浣纱果园';
-- 白月光
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,6.0,200.37556262697373,TRUE FROM p2rms15pepb5ciz.products WHERE name='白月光';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,7.0,NULL,175.3548475414474,TRUE FROM p2rms15pepb5ciz.products WHERE name='白月光';
-- 红岩2.0
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,68.71406926710526,TRUE FROM p2rms15pepb5ciz.products WHERE name='红岩2.0';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,62.25260685921054,TRUE FROM p2rms15pepb5ciz.products WHERE name='红岩2.0';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,56.14497472763158,TRUE FROM p2rms15pepb5ciz.products WHERE name='红岩2.0';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,53.02535405,TRUE FROM p2rms15pepb5ciz.products WHERE name='红岩2.0';
-- 红酒日晒
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,74.90468781973685,TRUE FROM p2rms15pepb5ciz.products WHERE name='红酒日晒';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,67.79717146447368,TRUE FROM p2rms15pepb5ciz.products WHERE name='红酒日晒';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,61.25883670131579,TRUE FROM p2rms15pepb5ciz.products WHERE name='红酒日晒';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,56.54731579999999,TRUE FROM p2rms15pepb5ciz.products WHERE name='红酒日晒';
-- 耶加雪菲G1
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,126.2020134118421,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G1';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,110.4345963986842,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G1';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,97.92669926710525,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G1';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,90.20965655,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G1';
-- 耶加雪菲G1（2024）
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,121.76979193653948,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G1（2024）';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,NULL,108.89113675881579,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G1（2024）';
-- 耶加雪菲G2（新产季）
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,90.9033342013158,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G2（新产季）';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,74.79718297763158,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G2（新产季）';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,66.39679637236841,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G2（新产季）';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,55.55380905,TRUE FROM p2rms15pepb5ciz.products WHERE name='耶加雪菲G2（新产季）';
-- 肯尼亚TOPAA
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,131.5839778855263,TRUE FROM p2rms15pepb5ciz.products WHERE name='肯尼亚TOPAA';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,119.44560034605263,TRUE FROM p2rms15pepb5ciz.products WHERE name='肯尼亚TOPAA';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,106.02372953026315,TRUE FROM p2rms15pepb5ciz.products WHERE name='肯尼亚TOPAA';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,97.62120655000001,TRUE FROM p2rms15pepb5ciz.products WHERE name='肯尼亚TOPAA';
-- 芒霜
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,121.011031175,TRUE FROM p2rms15pepb5ciz.products WHERE name='芒霜';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,NULL,105.929094425,TRUE FROM p2rms15pepb5ciz.products WHERE name='芒霜';
-- 莓屋
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,67.122195925,TRUE FROM p2rms15pepb5ciz.products WHERE name='莓屋';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,60.82686167500001,TRUE FROM p2rms15pepb5ciz.products WHERE name='莓屋';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,54.82998164868421,TRUE FROM p2rms15pepb5ciz.products WHERE name='莓屋';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,51.793425049999996,TRUE FROM p2rms15pepb5ciz.products WHERE name='莓屋';
-- 菠萝意式
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,100.2471022276316,TRUE FROM p2rms15pepb5ciz.products WHERE name='菠萝意式';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,NULL,87.90708653026316,TRUE FROM p2rms15pepb5ciz.products WHERE name='菠萝意式';
-- 酒心巧克力
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,74.90468781973685,TRUE FROM p2rms15pepb5ciz.products WHERE name='酒心巧克力';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,67.79717146447368,TRUE FROM p2rms15pepb5ciz.products WHERE name='酒心巧克力';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,61.25883670131579,TRUE FROM p2rms15pepb5ciz.products WHERE name='酒心巧克力';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,56.54731579999999,TRUE FROM p2rms15pepb5ciz.products WHERE name='酒心巧克力';
-- 金色山脉
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,2.0,13.0,66.06094703026316,TRUE FROM p2rms15pepb5ciz.products WHERE name='金色山脉';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,14.0,23.0,59.87636488552633,TRUE FROM p2rms15pepb5ciz.products WHERE name='金色山脉';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,24.0,47.0,53.95331959605263,TRUE FROM p2rms15pepb5ciz.products WHERE name='金色山脉';
INSERT INTO p2rms15pepb5ciz.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) SELECT id,48.0,NULL,50.972139049999996,TRUE FROM p2rms15pepb5ciz.products WHERE name='金色山脉';
COMMIT;
