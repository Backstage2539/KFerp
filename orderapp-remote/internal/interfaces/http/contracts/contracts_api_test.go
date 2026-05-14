package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	contractsapp "orderapp/internal/application/contracts"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type fakeContractService struct {
	uploadCmd  contractsapp.UploadContractCommand
	uploadErr  error
	stampCmd   contractsapp.SaveStampedPDFCommand
	stampErr   error
	pdfFile    contractsapp.ContractFile
	latestFile contractsapp.ContractFile
}

func (s *fakeContractService) ListContracts(ctx context.Context) ([]contractsapp.ContractDocument, error) {
	return []contractsapp.ContractDocument{{ID: 7, Title: "合作合同", SourceKind: contractsapp.ContractSourcePDF, PDFURL: "/contracts/7/pdf"}}, nil
}

func (s *fakeContractService) UploadContract(ctx context.Context, cmd contractsapp.UploadContractCommand) (contractsapp.ContractDocument, error) {
	s.uploadCmd = cmd
	if s.uploadErr != nil {
		return contractsapp.ContractDocument{}, s.uploadErr
	}
	return contractsapp.ContractDocument{ID: 7, Title: "合作合同", SourceFilename: cmd.Filename, SourceKind: contractsapp.ContractSourcePDF, PDFURL: "/contracts/7/pdf"}, nil
}

func (s *fakeContractService) SaveStampedPDF(ctx context.Context, cmd contractsapp.SaveStampedPDFCommand) (contractsapp.ContractStampedVersion, error) {
	s.stampCmd = cmd
	if s.stampErr != nil {
		return contractsapp.ContractStampedVersion{}, s.stampErr
	}
	return contractsapp.ContractStampedVersion{ID: 11, ContractID: cmd.ContractID, VersionNo: 1, SealAssetID: cmd.SealAssetID, DownloadURL: "/contracts/7/stamped/11.pdf", IsLatest: true}, nil
}

func (s *fakeContractService) LoadContractPDFFile(ctx context.Context, contractID int64) (contractsapp.ContractFile, error) {
	return s.pdfFile, nil
}

func (s *fakeContractService) LoadStampedPDFFile(ctx context.Context, contractID, versionID int64, latest bool) (contractsapp.ContractFile, error) {
	return s.latestFile, nil
}

func TestContractAPIUploadAndSaveStampedPDF(t *testing.T) {
	e := echo.New()
	svc := &fakeContractService{}
	RegisterRoutes(e, Dependencies{Contracts: svc})

	var uploadBody bytes.Buffer
	uploadWriter := multipart.NewWriter(&uploadBody)
	part, err := uploadWriter.CreateFormFile("file", "合作合同.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("%PDF-1.4\nbody")); err != nil {
		t.Fatal(err)
	}
	if err := uploadWriter.Close(); err != nil {
		t.Fatal(err)
	}
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/contracts", &uploadBody)
	uploadReq.Header.Set(echo.HeaderContentType, uploadWriter.FormDataContentType())
	uploadReq.Header.Set("X-Actor", "测试员")
	uploadRec := httptest.NewRecorder()
	e.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", uploadRec.Code, uploadRec.Body.String())
	}
	if svc.uploadCmd.Filename != "合作合同.pdf" || string(svc.uploadCmd.Data[:5]) != "%PDF-" {
		t.Fatalf("upload cmd = %+v", svc.uploadCmd)
	}

	var stampBody bytes.Buffer
	stampWriter := multipart.NewWriter(&stampBody)
	stampedPart, err := stampWriter.CreateFormFile("file", "合作合同-盖章.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stampedPart.Write([]byte("%PDF-1.4\nstamped")); err != nil {
		t.Fatal(err)
	}
	if err := stampWriter.WriteField("seal_asset_id", "9"); err != nil {
		t.Fatal(err)
	}
	if err := stampWriter.WriteField("placements", `[{"page_number":2,"x":48,"y":120,"width":96,"height":60}]`); err != nil {
		t.Fatal(err)
	}
	if err := stampWriter.Close(); err != nil {
		t.Fatal(err)
	}
	stampReq := httptest.NewRequest(http.MethodPost, "/api/contracts/7/stamped", &stampBody)
	stampReq.Header.Set(echo.HeaderContentType, stampWriter.FormDataContentType())
	stampRec := httptest.NewRecorder()
	e.ServeHTTP(stampRec, stampReq)
	if stampRec.Code != http.StatusOK {
		t.Fatalf("stamp status=%d body=%s", stampRec.Code, stampRec.Body.String())
	}
	if svc.stampCmd.ContractID != 7 || svc.stampCmd.SealAssetID != 9 || len(svc.stampCmd.Placements) != 1 || svc.stampCmd.Placements[0].PageNumber != 2 {
		t.Fatalf("stamp cmd = %+v", svc.stampCmd)
	}
}

func TestContractAPIDownloadsSourceAndStampedPDF(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.pdf")
	stamped := filepath.Join(dir, "stamped.pdf")
	if err := os.WriteFile(source, []byte("%PDF-1.4\nsource"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stamped, []byte("%PDF-1.4\nstamped"), 0644); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Contracts: &fakeContractService{
		pdfFile:    contractsapp.ContractFile{Path: source, Filename: "合作合同.pdf", ContentType: "application/pdf"},
		latestFile: contractsapp.ContractFile{Path: stamped, Filename: "合作合同-stamped-V1.pdf", ContentType: "application/pdf"},
	}})

	for _, item := range []struct {
		path     string
		filename string
		body     string
	}{
		{path: "/contracts/7/pdf", filename: "合作合同.pdf", body: "source"},
		{path: "/contracts/7/stamped-latest.pdf", filename: "合作合同-stamped-V1.pdf", body: "stamped"},
	} {
		req := httptest.NewRequest(http.MethodGet, item.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Header().Get(echo.HeaderContentType) != "application/pdf" || !strings.Contains(rec.Body.String(), item.body) {
			t.Fatalf("%s status=%d type=%q body=%q", item.path, rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.String())
		}
		if !strings.Contains(rec.Header().Get(echo.HeaderContentDisposition), item.filename) {
			t.Fatalf("%s disposition=%q", item.path, rec.Header().Get(echo.HeaderContentDisposition))
		}
	}
}

func TestContractAPIRoutesRegistered(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Contracts: &fakeContractService{}})
	routes := map[string]bool{}
	for _, route := range e.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /contracts",
		"GET /api/contracts",
		"POST /api/contracts",
		"GET /contracts/:id/pdf",
		"POST /api/contracts/:id/stamped",
		"GET /contracts/:id/stamped/:version_id.pdf",
		"GET /contracts/:id/stamped-latest.pdf",
	} {
		if !routes[want] {
			raw, _ := json.MarshalIndent(routes, "", "  ")
			t.Fatalf("missing route %s in %s", want, raw)
		}
	}
}

func TestParseContractIDRejectsInvalidValues(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/contracts/nope/pdf", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nope")
	if _, err := parseContractID(c); err == nil || !strings.Contains(err.Error(), "invalid contract id") {
		t.Fatalf("parse invalid id error = %v", err)
	}
	c.SetParamValues("42")
	got, err := parseContractID(c)
	if err != nil || got != 42 {
		t.Fatalf("parse valid id = %d, %v", got, err)
	}
	if _, err := parseContractVersionID("0.pdf"); err == nil {
		t.Fatal("parse invalid version id error = nil")
	}
	gotVersion, err := parseContractVersionID(fmt.Sprintf("%d.pdf", 12))
	if err != nil || gotVersion != 12 {
		t.Fatalf("parse version = %d, %v", gotVersion, err)
	}
}
