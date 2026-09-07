package costing

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type batchRepoFake struct {
	fakeRepo
	calls    int
	commands []PublishBeanListCommand
}

func (r *batchRepoFake) SaveBeanListBatch(_ context.Context, commands []PublishBeanListCommand, publish bool) ([]BeanListPublication, error) {
	r.calls++
	r.commands = commands
	rows := make([]BeanListPublication, len(commands))
	for i, cmd := range commands {
		rows[i] = BeanListPublication{ID: int64(i + 1), Version: cmd.Version, Config: cmd.Config, Content: cmd.Content}
	}
	return rows, nil
}

func batchFixture(count int) BeanListBatchCommand {
	cmd := BeanListBatchCommand{PublishBeanListCommand: PublishBeanListCommand{ListType: "commercial", Version: "V3.0.6", OwnerType: "official"}, DefaultTableKey: "table-1"}
	for i := 1; i <= count; i++ {
		cmd.Tables = append(cmd.Tables, BeanListBatchTable{Key: fmt.Sprintf("table-%d", i), Name: fmt.Sprintf("规格%d价格表", i), Config: map[string]any{}, Content: map[string]any{"groups": []any{map[string]any{"items": []any{map[string]any{"name": "测试商品"}}}}}})
	}
	return cmd
}

func TestBeanListBatchNamesDefaultAndAtomicValidation(t *testing.T) {
	for _, count := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			r := &batchRepoFake{}
			cmd := batchFixture(count)
			cmd.Tables[0].Name = "  227g价格表  "
			result, err := NewService(r).SaveBeanListDraftBatch(context.Background(), cmd)
			if err != nil {
				t.Fatal(err)
			}
			if r.calls != 1 || len(result.Tables) != count {
				t.Fatalf("batch=%+v calls=%d", result, r.calls)
			}
			for i, c := range r.commands {
				meta := BeanListBatchMetadata(c.Config)
				if c.Version != "V3.0.6" || meta.TableKey != fmt.Sprintf("table-%d", i+1) || meta.IsDefaultTable != (i == 0) {
					t.Fatalf("command=%+v meta=%+v", c, meta)
				}
			}
			if BeanListBatchMetadata(r.commands[0].Config).TableName != "227g价格表" {
				t.Fatal("name not trimmed")
			}
		})
	}
	for _, kind := range []string{"empty", "duplicate", "no-default", "duplicate-key", "no-tables", "invalid-last"} {
		t.Run(kind, func(t *testing.T) {
			r := &batchRepoFake{}
			cmd := batchFixture(3)
			switch kind {
			case "empty":
				cmd.Tables[1].Name = " "
			case "duplicate":
				cmd.Tables[1].Name = " " + cmd.Tables[0].Name + " "
			case "no-default":
				cmd.DefaultTableKey = "missing"
			case "duplicate-key":
				cmd.Tables[1].Key = cmd.Tables[0].Key
			case "no-tables":
				cmd.Tables = nil
			case "invalid-last":
				cmd.Tables[2].Config = map[string]any{"product_spec_selections": []any{map[string]any{"sku_id": -1}}}
			}
			_, err := NewService(r).SaveBeanListDraftBatch(context.Background(), cmd)
			if err == nil || r.calls != 0 {
				t.Fatalf("err=%v calls=%d", err, r.calls)
			}
		})
	}
}

func TestBeanListBatchRejectsEmptyPublishedTableAndDoesNotMutateInput(t *testing.T) {
	r := &batchRepoFake{}
	cmd := batchFixture(2)
	cmd.Tables[1].Content = map[string]any{}
	_, err := NewService(r).PublishBeanListBatch(context.Background(), cmd)
	if err == nil || !strings.Contains(err.Error(), "规格2价格表") || r.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, r.calls)
	}
	if _, ok := cmd.Tables[0].Config["publication_batch"]; ok {
		t.Fatal("input was mutated")
	}
}

func TestNamedPublicationPDFFilenameAndMetadata(t *testing.T) {
	row := BeanListPublication{ID: 23, ListType: "commercial", Version: "V3.0.6", Config: map[string]any{"publication_batch": map[string]any{"release_id": "batch-1", "table_key": "a", "table_name": "227g价格表", "is_default_table": true}}}
	if got := beanListPublicationPDFFilename(row); !strings.Contains(got, "227g价格表") || !strings.Contains(got, "V3.0.6") {
		t.Fatalf("filename=%s", got)
	}
	meta := BeanListBatchMetadata(row.Config)
	if meta.ReleaseID != "batch-1" || !meta.IsDefaultTable {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestNamedPublicationPDFCacheIdentityDoesNotCollideWithinVersion(t *testing.T) {
	keys := map[string]bool{}
	for _, id := range []int64{1, 2, 3} {
		key := beanListPublicationPDFCacheKey(BeanListPublication{ID: id, Version: "V3.0.6"})
		if keys[key] {
			t.Fatal("PDF cache collision")
		}
		keys[key] = true
	}
}
