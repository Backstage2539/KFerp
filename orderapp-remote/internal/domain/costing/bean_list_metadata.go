package costing

import "strings"

type excelBeanListMeta struct {
	name           string
	code           string
	category       string
	recommendedUse string
	flavor         string
	description    string
}

func commercialBeanListDisplay(name string) BeanListDisplay {
	return beanListDisplay(name, commercialBeanListMetadata)
}

func retailBeanListDisplay(name string) BeanListDisplay {
	return beanListDisplay(name, retailBeanListMetadata)
}

func beanListDisplay(name string, rows []excelBeanListMeta) BeanListDisplay {
	key := normalizeBeanListName(name)
	for _, row := range rows {
		if normalizeBeanListName(row.name) == key {
			return BeanListDisplay{
				Code:           row.code,
				Category:       row.category,
				DisplayName:    row.name,
				RecommendedUse: row.recommendedUse,
				Flavor:         row.flavor,
				Description:    row.description,
			}
		}
	}
	return BeanListDisplay{}
}

func normalizeBeanListName(s string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "（", "(", "）", ")")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(s)))
}

var commercialBeanListMetadata = []excelBeanListMeta{
	{
		name:     "曲奇拼配",
		code:     "1.1",
		category: "1、工厂量单",
		flavor:   "坚果、焦糖、巧克力曲奇",
	},
	{
		name:           "金色山脉",
		code:           "3.1",
		category:       "3、庄园精品豆：云南孟连兴福茶咖厂新产季精选",
		recommendedUse: "意式SOE",
		flavor:         "柑橘、坚果、焦糖、可可、饱满",
		description:    "甄选高海拔地块卡蒂姆，水洗处理、中深度烘焙",
	},
	{
		name:           "酒心巧克力",
		code:           "3.2",
		category:       "3、庄园精品豆：云南孟连兴福茶咖厂新产季精选",
		recommendedUse: "意式SOE",
		flavor:         "莓果、红酒、菠萝蜜、奶油",
		description:    "卡蒂姆日晒、中度烘焙（庄园差异性产品）",
	},
	{
		name:           "菠萝意式2.0",
		code:           "3.3",
		category:       "3、庄园精品豆：云南孟连兴福茶咖厂新产季精选",
		recommendedUse: "意式SOE",
		flavor:         "菠萝、奶油甜感、高强度风味表现",
		description:    "黄波旁厌氧日晒、中深度烘焙",
	},
	{
		name:           "橘皮乌龙",
		code:           "3.4",
		category:       "3、庄园精品豆：云南孟连兴福茶咖厂新产季精选",
		recommendedUse: "手冲",
		flavor:         "柑橘、白花、乌龙茶、黄糖",
		description:    "甄选高海拔地块卡蒂姆、水洗处理、浅度烘焙（清新柑橘酸质，顺滑干净）",
	},
	{
		name:           "芒霜2.0",
		code:           "3.5",
		category:       "3、庄园精品豆：云南孟连兴福茶咖厂新产季精选",
		recommendedUse: "手冲",
		flavor:         "威士忌，甜芒果、水果糖、果汁口感",
		description:    "高海拔全红果卡蒂姆厌氧日晒、浅度烘焙（2025COE竞赛级产品）",
	},
	{
		name:           "小菠萝2.0",
		code:           "3.6",
		category:       "3、庄园精品豆：云南孟连兴福茶咖厂新产季精选",
		recommendedUse: "手冲",
		flavor:         "青苹果、菠萝、果汁口感、干净度高",
		description:    "黄波旁厌氧日晒、浅度烘焙（2025COE入围前24名）",
	},
	{
		name:           "萨奇姆",
		code:           "3.7",
		category:       "3、庄园精品豆：云南孟连兴福茶咖厂新产季精选",
		recommendedUse: "手冲",
		flavor:         "柑橘、青苹果、白花、黄糖",
		description:    "萨奇姆水洗、浅度烘焙（云南明星豆种）",
	},
	{
		name:           "曜石2.0",
		code:           "4.1",
		category:       "4、精品意式拼配：",
		recommendedUse: "醇厚 饱满",
		flavor:         "烤坚果、黑可可 、奶油、高醇度、口感饱满",
		description:    "云南孟连&哥伦比亚&印尼、深度烘焙（高醇度的坚果巧克力调拼配）",
	},
	{
		name:           "红岩2.0",
		code:           "4.2",
		category:       "4、精品意式拼配：",
		recommendedUse: "微莓果酸\n焦糖甜",
		flavor:         "莓果、太妃糖、吐司、坚果",
		description:    "庄园拼配、中深度烘焙（果香、醇厚两不误，纯云南拼配）",
	},
	{
		name:           "初晓",
		code:           "4.3",
		category:       "4、精品意式拼配：",
		recommendedUse: "甜香拼配",
		flavor:         "花香、伯爵红茶、焦糖、醇厚顺滑",
		description:    "云南孟连&哥伦比亚&埃塞、中深度烘焙（甜感为主，均衡百搭）",
	},
	{
		name:           "松饼",
		code:           "4.4",
		category:       "4、精品意式拼配：",
		recommendedUse: "高醇度",
		flavor:         "坚果、黑巧克力、醇厚、奶油",
		description:    "云南孟连&哥伦比亚&巴西、深度烘焙",
	},
	{
		name:           "榛巧",
		code:           "4.5",
		category:       "4、精品意式拼配：",
		recommendedUse: "回味榛果\n饱满",
		flavor:         "甜巧克力、榛果、黑糖、顺滑口感",
		description:    "云南孟连&哥伦比亚&哥斯达黎加、深度烘焙（奶咖出品必选，榛果香气持久）",
	},
	{
		name:           "果语花香",
		code:           "4.6",
		category:       "4、精品意式拼配：",
		recommendedUse: "可手冲\n花果均衡",
		flavor:         "花香、荔枝、蜜桃、黄糖",
		description:    "云南孟连&哥斯达黎加&埃塞、中浅度烘焙（果酸花香兼具）",
	},
	{
		name:           "耶加雪菲G2",
		code:           "5.1",
		category:       "5、原产地精选豆：",
		recommendedUse: "手冲/SOE/冷萃",
		flavor:         "茉莉、柑橘、果茶口感",
		description:    "埃塞·耶加雪菲，原生种水洗、中度烘焙",
	},
	{
		name:           "Uraga乌拉嘎",
		code:           "5.2",
		category:       "5、原产地精选豆：",
		recommendedUse: "手冲/SOE/冷萃",
		flavor:         "明显的花香、柑橘、荔枝，红糖甜，绿茶",
		description:    "埃塞·古吉·Uraga、74112水洗处理、浅度烘焙（新产季埃塞水洗）",
	},
	{
		name:           "浣纱果园",
		code:           "5.3",
		category:       "5、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "玫瑰花香、葡萄、橙色莓果、草莓",
		description:    "埃塞·古吉·夏奇索、原生种、日晒处理、浅度烘焙（折扣中）",
	},
	{
		name:           "肯尼亚TOPAA",
		code:           "5.4",
		category:       "5、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "小番茄、乌梅、红糖",
		description:    "肯尼亚·涅里山庄 TOPAA SL28水洗处理、浅度烘焙（喜酸必选）（折扣中）",
	},
	{
		name:           "森林瑰夏",
		code:           "5.5",
		category:       "5、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "茉莉花、柑橘、柠檬、黄糖",
		description:    "埃塞·班奇玛吉、Gesha 水洗G1、浅度烘焙（平价瑰夏）",
	},
	{
		name:           "Nenka嫩咖",
		code:           "5.6",
		category:       "5、原产地精选豆：",
		recommendedUse: "手冲/冰滴/冷萃",
		flavor:         "水蜜桃、百香果、樱桃、草莓、甜感哇塞",
		description:    "埃塞·古吉·Nenka、74158日晒处理、浅度烘焙（新产季埃塞日晒）",
	},
	{
		name:           "曼特宁",
		code:           "5.7",
		category:       "5、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "雪松、杉木、 巧克力",
		description:    "印尼·苏门答腊 卡蒂姆&铁皮卡 湿刨处理、深度烘焙",
	},
	{
		name:           "白月光-瑰夏",
		code:           "6.1",
		category:       "6、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "绿茶、馥郁花香、橘子软糖、果汁口感",
		description:    "洪都拉斯·圣巴巴拉 Geisha水洗 浅度烘焙（极致的瑰夏体验）",
	},
	{
		name:           "芸上莓梦",
		code:           "6.2",
		category:       "6、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "明亮莓果酸、油桃、蜜饯、复合果汁",
		description:    "埃塞·西达摩 74158 日晒处理 浅度烘焙（如目达摩 TOH#5）",
	},
	{
		name:           "晨曦-娜伊",
		code:           "6.3",
		category:       "6、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "咖啡花、草莓、李子、玫瑰酒",
		description:    "秘鲁 VillaRica Oxapampa Pasco 日晒处理 浅度烘焙（COE#2）",
	},
	{
		name:           "晚香玉",
		code:           "6.4",
		category:       "6、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "坚果、香料、青苹果、熟葡萄、明亮酸质",
		description:    "印尼·苏拉威西岛 铁皮卡 水洗处理 中深度烘焙（印尼“蓝山”）",
	},
}

var retailBeanListMetadata = []excelBeanListMeta{
	{
		name:           "金色山脉",
		code:           "1.1",
		category:       "1、庄园精品豆：孟连兴福茶咖厂精选",
		recommendedUse: "意式SOE",
		flavor:         "柑橘、坚果、焦糖、可可、饱满",
		description:    "甄选高海拔地块卡蒂姆，水洗处理、中深度烘焙",
	},
	{
		name:           "酒心巧克力",
		code:           "1.2",
		category:       "1、庄园精品豆：孟连兴福茶咖厂精选",
		recommendedUse: "意式SOE",
		flavor:         "莓果、红酒、菠萝蜜、奶油",
		description:    "卡蒂姆日晒、中度烘焙（庄园差异性产品）",
	},
	{
		name:           "菠萝意式2.0",
		code:           "1.3",
		category:       "1、庄园精品豆：孟连兴福茶咖厂精选",
		recommendedUse: "意式SOE",
		flavor:         "菠萝、奶油甜感、高强度风味表现",
		description:    "黄波旁厌氧日晒、中深度烘焙",
	},
	{
		name:           "橘皮乌龙",
		code:           "1.4",
		category:       "1、庄园精品豆：孟连兴福茶咖厂精选",
		recommendedUse: "手冲",
		flavor:         "柑橘、白花、乌龙茶、黄糖",
		description:    "甄选高海拔地块卡蒂姆、浅度烘焙（清新柑橘酸质，顺滑干净）",
	},
	{
		name:           "芒霜2.0",
		code:           "1.5",
		category:       "1、庄园精品豆：孟连兴福茶咖厂精选",
		recommendedUse: "手冲",
		flavor:         "威士忌，甜芒果、水果糖、果汁口感",
		description:    "高海拔全红果卡蒂姆厌氧日晒、浅度烘焙（2025COE竞赛级产品）",
	},
	{
		name:           "小菠萝2.0",
		code:           "1.6",
		category:       "1、庄园精品豆：孟连兴福茶咖厂精选",
		recommendedUse: "手冲",
		flavor:         "青苹果、菠萝、果汁口感、干净度高",
		description:    "黄波旁厌氧日晒、浅度烘焙（2025COE入围前24名）",
	},
	{
		name:           "萨奇姆",
		code:           "1.7",
		category:       "1、庄园精品豆：孟连兴福茶咖厂精选",
		recommendedUse: "手冲",
		flavor:         "柑橘、青苹果、白花、黄糖",
		description:    "萨奇姆水洗、浅度烘焙（云南明星豆种）",
	},
	{
		name:           "曜石2.0",
		code:           "2.1",
		category:       "2、精品意式拼配：",
		recommendedUse: "醇厚 饱满",
		flavor:         "烤坚果、黑可可 、奶油、高醇度、口感饱满",
		description:    "云南孟连&哥伦比亚&印尼、深度烘焙（高醇度的坚果巧克力调拼配）",
	},
	{
		name:           "红岩2.0",
		code:           "2.2",
		category:       "2、精品意式拼配：",
		recommendedUse: "微莓果酸\n焦糖甜",
		flavor:         "莓果、太妃糖、吐司、坚果",
		description:    "庄园拼配、中深度烘焙（果香、醇厚两不误，纯云南拼配）",
	},
	{
		name:           "初晓",
		code:           "2.3",
		category:       "2、精品意式拼配：",
		recommendedUse: "甜香拼配",
		flavor:         "花香、伯爵红茶、焦糖、醇厚顺滑",
		description:    "云南孟连&哥伦比亚&埃塞、中深度烘焙（甜感为主，均衡百搭）",
	},
	{
		name:           "松饼",
		code:           "2.4",
		category:       "2、精品意式拼配：",
		recommendedUse: "高醇度",
		flavor:         "坚果、黑巧克力、醇厚、奶油",
		description:    "云南孟连&哥伦比亚&巴西、深度烘焙",
	},
	{
		name:           "榛巧",
		code:           "2.5",
		category:       "2、精品意式拼配：",
		recommendedUse: "回味榛果\n饱满",
		flavor:         "甜巧克力、榛果、黑糖、顺滑口感",
		description:    "云南孟连&哥伦比亚&哥斯达黎加、深度烘焙（奶咖出品必选，榛果香气持久）",
	},
	{
		name:           "果语花香",
		code:           "2.6",
		category:       "2、精品意式拼配：",
		recommendedUse: "可手冲\n花果均衡",
		flavor:         "花香、荔枝、蜜桃、黄糖",
		description:    "云南孟连&哥斯达黎加&埃塞、中浅度烘焙（果酸花香兼具）",
	},
	{
		name:           "耶加雪菲G2",
		code:           "3.1",
		category:       "3、原产地精选豆：",
		recommendedUse: "手冲/SOE/冷萃",
		flavor:         "茉莉、柑橘、果茶口感",
		description:    "埃塞·耶加雪菲，原生种水洗、中度烘焙",
	},
	{
		name:           "Uraga乌拉嘎",
		code:           "3.2",
		category:       "3、原产地精选豆：",
		recommendedUse: "手冲/SOE/冷萃",
		flavor:         "明显的花香、柑橘、荔枝，红糖甜，绿茶",
		description:    "埃塞·古吉·Uraga、74112水洗处理、浅度烘焙（新产季埃塞水洗）",
	},
	{
		name:           "浣纱果园",
		code:           "3.3",
		category:       "3、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "玫瑰花香、葡萄、橙色莓果、草莓",
		description:    "埃塞·古吉·夏奇索、原生种、日晒处理、浅度烘焙（折扣中）",
	},
	{
		name:           "肯尼亚TOPAA",
		code:           "3.4",
		category:       "3、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "小番茄、乌梅、红糖",
		description:    "肯尼亚·涅里山庄 TOPAA SL28水洗处理、浅度烘焙（喜酸必选）（折扣中）",
	},
	{
		name:           "森林瑰夏",
		code:           "3.5",
		category:       "3、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "茉莉花、柑橘、柠檬、黄糖",
		description:    "埃塞·班奇玛吉、Gesha 水洗G1、浅度烘焙（平价瑰夏）",
	},
	{
		name:           "Nenka嫩咖",
		code:           "3.6",
		category:       "3、原产地精选豆：",
		recommendedUse: "手冲/冰滴/冷萃",
		flavor:         "水蜜桃、百香果、樱桃、草莓、甜感哇塞",
		description:    "埃塞·古吉·Nenka、74158日晒处理、浅度烘焙（新产季埃塞日晒）",
	},
	{
		name:           "曼特宁",
		code:           "3.7",
		category:       "3、原产地精选豆：",
		recommendedUse: "手冲",
		flavor:         "雪松、杉木、 巧克力",
		description:    "印尼·苏门答腊 卡蒂姆&铁皮卡 湿刨处理、深度烘焙",
	},
	{
		name:           "白月光-瑰夏",
		code:           "4.1",
		category:       "4、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "绿茶、馥郁花香、橘子软糖、果汁口感",
		description:    "洪都拉斯·圣巴巴拉 Geisha水洗 浅度烘焙（极致的瑰夏体验）",
	},
	{
		name:           "芸上莓梦",
		code:           "4.2",
		category:       "4、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "明亮莓果酸、油桃、蜜饯、复合果汁",
		description:    "埃塞·西达摩 74158 日晒处理 浅度烘焙（如目达摩 TOH#5）",
	},
	{
		name:           "晨曦-娜伊",
		code:           "4.3",
		category:       "4、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "咖啡花、草莓、李子、玫瑰酒",
		description:    "秘鲁 VillaRica Oxapampa Pasco 日晒处理 浅度烘焙（COE#2）",
	},
	{
		name:           "晚香玉",
		code:           "4.4",
		category:       "4、差异性爆款：",
		recommendedUse: "手冲",
		flavor:         "坚果、香料、青苹果、熟葡萄、明亮酸质",
		description:    "印尼·苏拉威西岛 铁皮卡 水洗处理 中深度烘焙（印尼“蓝山”）",
	},
}
