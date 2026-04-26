package production

import (
	"os"
	"strings"
	"testing"
)

func TestMaterialsRawBeans20260424SQL(t *testing.T) {
	b, err := os.ReadFile("db/materials_raw_beans_20260424.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)

	required := []string{
		"material_import_20260424",
		"('卡蒂姆水洗', 54.000)",
		"('白月光', 54.000)",
		"('乌拉嘎', 108.000)",
		"('森林瑰夏日晒', 0.000)",
		"UPDATE p2rms15pepb5ciz.materials m",
		"WHERE m.name = i.name",
		"'bean-' || substr(md5(i.name), 1, 10)",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("materials import SQL missing %q", want)
		}
	}
}
