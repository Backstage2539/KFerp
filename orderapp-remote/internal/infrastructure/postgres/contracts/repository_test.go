package contracts

import (
	"context"
	"fmt"
	contractsapp "orderapp/internal/application/contracts"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestContractRepositoryCreatesListsAndLoadsStampedVersions(t *testing.T) {
	pool, schema := newContractsPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	assetDir := t.TempDir()
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	repo := NewRepository(pool, schema, WithAssetDir(assetDir))

	doc, err := repo.CreateContract(ctx, contractsapp.CreateContractRecord{
		Actor:             "测试员",
		Title:             "合作合同",
		SourceFilename:    "合作合同.pdf",
		SourceContentType: "application/pdf",
		SourceKind:        contractsapp.ContractSourcePDF,
		SourceObjectKey:   "contracts/source/source.pdf",
		SourceBytes:       15,
		SourceSHA256:      "source-sha",
		PDFObjectKey:      "contracts/source/source.pdf",
		PDFBytes:          15,
		PDFSHA256:         "pdf-sha",
	})
	if err != nil {
		t.Fatalf("CreateContract: %v", err)
	}
	first, err := repo.SaveStampedVersion(ctx, contractsapp.SaveStampedVersionRecord{
		Actor:       "盖章员",
		ContractID:  doc.ID,
		SealAssetID: 3,
		Placements:  []contractsapp.StampPlacement{{PageNumber: 1, X: 10, Y: 20, Width: 80, Height: 50}},
		ObjectKey:   "contracts/stamped/1/V1.pdf",
		Bytes:       30,
		SHA256:      "v1",
	})
	if err != nil {
		t.Fatalf("SaveStampedVersion first: %v", err)
	}
	second, err := repo.SaveStampedVersion(ctx, contractsapp.SaveStampedVersionRecord{
		Actor:       "盖章员",
		ContractID:  doc.ID,
		SealAssetID: 4,
		Placements:  []contractsapp.StampPlacement{{PageNumber: 2, X: 30, Y: 40, Width: 90, Height: 55}},
		ObjectKey:   "contracts/stamped/1/V2.pdf",
		Bytes:       40,
		SHA256:      "v2",
	})
	if err != nil {
		t.Fatalf("SaveStampedVersion second: %v", err)
	}
	if first.VersionNo != 1 || first.IsLatest || second.VersionNo != 2 || !second.IsLatest {
		t.Fatalf("versions first=%+v second=%+v", first, second)
	}
	rows, err := repo.ListContracts(ctx)
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if len(rows) != 1 || rows[0].LatestStamped == nil || rows[0].LatestStamped.ID != second.ID {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].PDFURL != fmt.Sprintf("/contracts/%d/pdf", doc.ID) || rows[0].LatestStamped.DownloadURL != fmt.Sprintf("/contracts/%d/stamped/%d.pdf", doc.ID, second.ID) {
		t.Fatalf("download urls row=%+v latest=%+v", rows[0], rows[0].LatestStamped)
	}
	pdfFile, err := repo.LoadContractPDFFile(ctx, doc.ID)
	if err != nil {
		t.Fatalf("LoadContractPDFFile: %v", err)
	}
	if !strings.HasSuffix(pdfFile.Path, "contracts/source/source.pdf") || pdfFile.Filename != "合作合同.pdf" || pdfFile.ContentType != "application/pdf" {
		t.Fatalf("pdf file = %+v", pdfFile)
	}
	latest, err := repo.LoadStampedPDFFile(ctx, doc.ID, 0, true)
	if err != nil {
		t.Fatalf("LoadStampedPDFFile latest: %v", err)
	}
	if !strings.HasSuffix(latest.Path, "contracts/stamped/1/V2.pdf") || latest.Filename != "合作合同-stamped-V2.pdf" {
		t.Fatalf("latest stamped file = %+v", latest)
	}
	history, err := repo.LoadStampedPDFFile(ctx, doc.ID, first.ID, false)
	if err != nil {
		t.Fatalf("LoadStampedPDFFile history: %v", err)
	}
	if history.Filename != "合作合同-stamped-V1.pdf" {
		t.Fatalf("history stamped file = %+v", history)
	}
}

func newContractsPostgresTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for contracts postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_contracts_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}
