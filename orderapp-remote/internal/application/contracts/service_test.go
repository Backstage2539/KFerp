package contracts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	created        CreateContractRecord
	createCalls    int
	stamped        SaveStampedVersionRecord
	stampedCalls   int
	createErr      error
	saveStampedErr error
	nextContractID int64
	nextStampedID  int64
}

func (r *fakeRepository) CreateContract(ctx context.Context, record CreateContractRecord) (ContractDocument, error) {
	r.createCalls++
	r.created = record
	if r.createErr != nil {
		return ContractDocument{}, r.createErr
	}
	id := r.nextContractID
	if id == 0 {
		id = 77
	}
	return ContractDocument{
		ID:                id,
		Title:             record.Title,
		SourceFilename:    record.SourceFilename,
		SourceContentType: record.SourceContentType,
		SourceKind:        record.SourceKind,
		SourceObjectKey:   record.SourceObjectKey,
		PDFObjectKey:      record.PDFObjectKey,
		PDFBytes:          record.PDFBytes,
		CreatedBy:         record.Actor,
	}, nil
}

func (r *fakeRepository) SaveStampedVersion(ctx context.Context, record SaveStampedVersionRecord) (ContractStampedVersion, error) {
	r.stampedCalls++
	r.stamped = record
	if r.saveStampedErr != nil {
		return ContractStampedVersion{}, r.saveStampedErr
	}
	id := r.nextStampedID
	if id == 0 {
		id = 12
	}
	return ContractStampedVersion{
		ID:          id,
		ContractID:  record.ContractID,
		VersionNo:   1,
		SealAssetID: record.SealAssetID,
		ObjectKey:   record.ObjectKey,
		Bytes:       record.Bytes,
		IsLatest:    true,
		CreatedBy:   record.Actor,
	}, nil
}

func (r *fakeRepository) ListContracts(ctx context.Context) ([]ContractDocument, error) {
	return nil, nil
}
func (r *fakeRepository) LoadContractPDFFile(ctx context.Context, contractID int64) (ContractFile, error) {
	return ContractFile{}, nil
}
func (r *fakeRepository) LoadStampedPDFFile(ctx context.Context, contractID, versionID int64, latest bool) (ContractFile, error) {
	return ContractFile{}, nil
}

type fakeConverter struct {
	calls      int
	sourcePath string
	outputDir  string
}

func (c *fakeConverter) ConvertDOCXToPDF(ctx context.Context, sourcePath, outputDir string) (string, error) {
	c.calls++
	c.sourcePath = sourcePath
	c.outputDir = outputDir
	out := filepath.Join(outputDir, "converted.pdf")
	if err := os.WriteFile(out, []byte("%PDF-1.4\nconverted"), 0644); err != nil {
		return "", err
	}
	return out, nil
}

func TestUploadPDFContractStoresOriginalAsStampablePDF(t *testing.T) {
	dir := t.TempDir()
	repo := &fakeRepository{}
	svc := NewService(repo, &fakeConverter{}, WithAssetDir(dir), WithClock(func() time.Time {
		return time.Unix(1778766000, 0).UTC()
	}))

	doc, err := svc.UploadContract(context.Background(), UploadContractCommand{
		Actor:       " 测试员 ",
		Filename:    "合作合同.pdf",
		ContentType: "application/pdf",
		Data:        []byte("%PDF-1.4\nbody"),
	})
	if err != nil {
		t.Fatalf("UploadContract pdf: %v", err)
	}
	if doc.ID != 77 || repo.createCalls != 1 {
		t.Fatalf("created doc=%+v createCalls=%d", doc, repo.createCalls)
	}
	if repo.created.Actor != "测试员" || repo.created.SourceKind != ContractSourcePDF {
		t.Fatalf("created record actor/source = %q/%q", repo.created.Actor, repo.created.SourceKind)
	}
	if repo.created.SourceObjectKey == "" || repo.created.PDFObjectKey != repo.created.SourceObjectKey {
		t.Fatalf("pdf upload should reuse original PDF object key, got source=%q pdf=%q", repo.created.SourceObjectKey, repo.created.PDFObjectKey)
	}
	if _, err := os.Stat(filepath.Join(dir, repo.created.PDFObjectKey)); err != nil {
		t.Fatalf("stored pdf missing: %v", err)
	}
}

func TestUploadDOCXContractConvertsToPDFBeforePersistence(t *testing.T) {
	dir := t.TempDir()
	repo := &fakeRepository{}
	converter := &fakeConverter{}
	svc := NewService(repo, converter, WithAssetDir(dir), WithClock(func() time.Time {
		return time.Unix(1778766000, 0).UTC()
	}))

	doc, err := svc.UploadContract(context.Background(), UploadContractCommand{
		Actor:       "测试员",
		Filename:    "代加工合同.docx",
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Data:        []byte("PK\x03\x04docx"),
	})
	if err != nil {
		t.Fatalf("UploadContract docx: %v", err)
	}
	if doc.SourceKind != ContractSourceDOCX || converter.calls != 1 {
		t.Fatalf("source kind/calls = %q/%d", doc.SourceKind, converter.calls)
	}
	if repo.created.SourceObjectKey == "" || repo.created.PDFObjectKey == "" || repo.created.SourceObjectKey == repo.created.PDFObjectKey {
		t.Fatalf("docx upload should store distinct source/pdf object keys, got source=%q pdf=%q", repo.created.SourceObjectKey, repo.created.PDFObjectKey)
	}
	if !strings.HasSuffix(converter.sourcePath, ".docx") || converter.outputDir == "" {
		t.Fatalf("converter source/output = %q/%q", converter.sourcePath, converter.outputDir)
	}
	if repo.created.PDFBytes <= 0 {
		t.Fatalf("converted pdf bytes not recorded: %+v", repo.created)
	}
}

func TestUploadContractRejectsUnsupportedFileTypeWithoutPersisting(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo, &fakeConverter{}, WithAssetDir(t.TempDir()))

	_, err := svc.UploadContract(context.Background(), UploadContractCommand{
		Actor:       "测试员",
		Filename:    "合同.txt",
		ContentType: "text/plain",
		Data:        []byte("not a contract"),
	})
	if err == nil || !strings.Contains(err.Error(), "pdf or docx required") {
		t.Fatalf("unsupported error = %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("unsupported upload persisted record: %+v", repo.created)
	}
}

func TestUploadContractRejectsInvalidPDFBytesWithoutPersisting(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo, &fakeConverter{}, WithAssetDir(t.TempDir()))

	_, err := svc.UploadContract(context.Background(), UploadContractCommand{
		Actor:       "测试员",
		Filename:    "合同.pdf",
		ContentType: "application/pdf",
		Data:        []byte("not a pdf"),
	})
	if err == nil || !strings.Contains(err.Error(), "pdf file invalid") {
		t.Fatalf("invalid pdf error = %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("invalid pdf persisted record: %+v", repo.created)
	}
}

func TestSaveStampedPDFStoresVersionWithPlacements(t *testing.T) {
	dir := t.TempDir()
	repo := &fakeRepository{}
	svc := NewService(repo, &fakeConverter{}, WithAssetDir(dir), WithClock(func() time.Time {
		return time.Unix(1778766000, 0).UTC()
	}))

	version, err := svc.SaveStampedPDF(context.Background(), SaveStampedPDFCommand{
		Actor:       "盖章员",
		ContractID:  77,
		SealAssetID: 9,
		Data:        []byte("%PDF-1.4\nstamped"),
		Placements: []StampPlacement{{
			PageNumber: 2,
			X:          48,
			Y:          120,
			Width:      96,
			Height:     60,
		}},
	})
	if err != nil {
		t.Fatalf("SaveStampedPDF: %v", err)
	}
	if version.ID != 12 || repo.stampedCalls != 1 {
		t.Fatalf("version=%+v calls=%d", version, repo.stampedCalls)
	}
	if repo.stamped.ContractID != 77 || repo.stamped.SealAssetID != 9 || len(repo.stamped.Placements) != 1 {
		t.Fatalf("stamped record mismatch: %+v", repo.stamped)
	}
	if _, err := os.Stat(filepath.Join(dir, repo.stamped.ObjectKey)); err != nil {
		t.Fatalf("stamped pdf missing: %v", err)
	}
}
