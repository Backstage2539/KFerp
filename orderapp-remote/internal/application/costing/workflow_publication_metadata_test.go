package costing

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeBeanListCommandRestoresOnlyTrustedWorkflowTableMetadata(t *testing.T) {
	cmd, err := normalizeBeanListCommand(PublishBeanListCommand{
		ListType: "commercial", Version: "V3.0.7", OwnerType: "official",
		Config: map[string]any{"publication_batch": map[string]any{"table_key": "forged", "table_name": "伪造表"}},
		WorkflowPublicationMetadata: &PublicationTableMetadata{
			ReleaseID: "product-creator-run-9-price", TableKey: "table-2", TableName: "装配件报价",
			IsDefaultTable: false, DirectShipEnabled: true,
			CopyRequestID: "must-not-be-preserved", CopySourcePublicationID: 999,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	meta := BeanListBatchMetadata(cmd.Config)
	if meta.ReleaseID != "product-creator-run-9-price" || meta.TableKey != "table-2" || meta.TableName != "装配件报价" || meta.IsDefaultTable || !meta.DirectShipEnabled {
		t.Fatalf("trusted publication metadata was not preserved: %+v", meta)
	}
	if meta.CopyRequestID != "" || meta.CopySourcePublicationID != 0 {
		t.Fatalf("copy audit metadata must not be inherited: %+v", meta)
	}
	if cmd.WorkflowPublicationMetadata != nil {
		t.Fatal("internal metadata must be consumed during normalization")
	}
	body, err := json.Marshal(PublishBeanListCommand{WorkflowPublicationMetadata: &PublicationTableMetadata{TableKey: "private"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "workflow_publication_metadata") || strings.Contains(string(body), "private") {
		t.Fatalf("internal workflow metadata leaked to JSON: %s", body)
	}
}

func TestNormalizeBeanListCommandDropsUntrustedPublicationMetadata(t *testing.T) {
	cmd, err := normalizeBeanListCommand(PublishBeanListCommand{
		ListType: "commercial", Version: "V3.0.7", OwnerType: "official",
		Config: map[string]any{"publication_batch": map[string]any{"table_key": "forged", "table_name": "伪造表"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cmd.Config["publication_batch"]; ok {
		t.Fatalf("untrusted publication metadata must be discarded: %#v", cmd.Config["publication_batch"])
	}
}
