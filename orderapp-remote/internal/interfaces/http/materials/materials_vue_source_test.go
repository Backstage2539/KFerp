package materials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialsViewUsesModalForBeanProfile(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"openBeanProfileDialog",
		"profile-modal",
		"保存咖啡豆信息",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("MaterialsView.vue missing %q", want)
		}
	}
	if strings.Contains(src, `<td class="profile-cell">
                <div v-if="row.kind === 'bean'" class="bean-profile-grid">`) {
		t.Fatalf("bean profile editor is still inline in the table")
	}
}
