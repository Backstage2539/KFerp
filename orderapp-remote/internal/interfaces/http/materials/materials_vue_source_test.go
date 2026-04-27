package materials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialsViewUsesMasterDetailForTypeSpecificProfiles(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"materials-layout",
		"material-list-panel",
		"material-detail-panel",
		"selectMaterial(row)",
		"copySelectedMaterial",
		"deprecateSelectedMaterial",
		"bean_profile",
		"pack_profile",
		"包材属性",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("MaterialsView.vue missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"<th>编码</th>",
		"<th>单位</th>",
		`v-model.trim="row.code"`,
		`v-model.trim="row.name"`,
		"profile-modal",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("MaterialsView.vue still contains table inline edit or modal marker %q", forbidden)
		}
	}
}
