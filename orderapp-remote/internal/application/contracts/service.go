package contracts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ContractSourcePDF  = "pdf"
	ContractSourceDOCX = "docx"

	maxContractUploadBytes = 30 << 20
)

type PDFConverter interface {
	ConvertDOCXToPDF(ctx context.Context, sourcePath, outputDir string) (string, error)
}

type Repository interface {
	CreateContract(ctx context.Context, record CreateContractRecord) (ContractDocument, error)
	UpdateContract(ctx context.Context, record UpdateContractRecord) (ContractDocument, error)
	DeleteContract(ctx context.Context, record DeleteContractRecord) error
	SaveStampedVersion(ctx context.Context, record SaveStampedVersionRecord) (ContractStampedVersion, error)
	ListContracts(ctx context.Context) ([]ContractDocument, error)
	LoadContractPDFFile(ctx context.Context, contractID int64) (ContractFile, error)
	LoadStampedPDFFile(ctx context.Context, contractID, versionID int64, latest bool) (ContractFile, error)
}

type Service struct {
	repo      Repository
	converter PDFConverter
	assetDir  string
	clock     func() time.Time
}

type Option func(*Service)

func WithAssetDir(assetDir string) Option {
	return func(s *Service) {
		if strings.TrimSpace(assetDir) != "" {
			s.assetDir = strings.TrimSpace(assetDir)
		}
	}
}

func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func NewService(repo Repository, converter PDFConverter, opts ...Option) *Service {
	s := &Service{
		repo:      repo,
		converter: converter,
		assetDir:  "/app/data/assets",
		clock:     time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type UploadContractCommand struct {
	Actor       string
	Filename    string
	ContentType string
	Data        []byte
}

type CreateContractRecord struct {
	Actor             string
	Title             string
	SourceFilename    string
	SourceContentType string
	SourceKind        string
	SourceObjectKey   string
	SourceBytes       int64
	SourceSHA256      string
	PDFObjectKey      string
	PDFBytes          int64
	PDFSHA256         string
}

type UpdateContractCommand struct {
	Actor      string
	ContractID int64
	Title      string
	Note       string
}

type DeleteContractCommand struct {
	Actor      string
	ContractID int64
}

type UpdateContractRecord struct {
	Actor      string
	ContractID int64
	Title      string
	Note       string
}

type DeleteContractRecord struct {
	Actor      string
	ContractID int64
}

type ContractDocument struct {
	ID                int64                   `json:"id"`
	Title             string                  `json:"title"`
	Note              string                  `json:"note"`
	SourceFilename    string                  `json:"source_filename"`
	SourceContentType string                  `json:"source_content_type"`
	SourceKind        string                  `json:"source_kind"`
	SourceObjectKey   string                  `json:"source_object_key"`
	PDFObjectKey      string                  `json:"pdf_object_key"`
	PDFBytes          int64                   `json:"pdf_bytes"`
	PDFURL            string                  `json:"pdf_url"`
	LatestStamped     *ContractStampedVersion `json:"latest_stamped,omitempty"`
	CreatedAt         string                  `json:"created_at"`
	CreatedBy         string                  `json:"created_by"`
	DeletedAt         string                  `json:"deleted_at,omitempty"`
	DeletedBy         string                  `json:"deleted_by,omitempty"`
}

type SaveStampedPDFCommand struct {
	Actor       string
	ContractID  int64
	SealAssetID int64
	Data        []byte
	Placements  []StampPlacement
}

type SaveStampedVersionRecord struct {
	Actor       string
	ContractID  int64
	SealAssetID int64
	Placements  []StampPlacement
	ObjectKey   string
	Bytes       int64
	SHA256      string
}

type ContractStampedVersion struct {
	ID          int64            `json:"id"`
	ContractID  int64            `json:"contract_id"`
	VersionNo   int              `json:"version_no"`
	SealAssetID int64            `json:"seal_asset_id"`
	Placements  []StampPlacement `json:"placements"`
	ObjectKey   string           `json:"object_key"`
	Bytes       int64            `json:"bytes"`
	DownloadURL string           `json:"download_url"`
	IsLatest    bool             `json:"is_latest"`
	CreatedAt   string           `json:"created_at"`
	CreatedBy   string           `json:"created_by"`
}

type StampPlacement struct {
	PageNumber int     `json:"page_number"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
}

type ContractFile struct {
	Path        string
	Filename    string
	ContentType string
}

func (s *Service) ListContracts(ctx context.Context) ([]ContractDocument, error) {
	return s.repo.ListContracts(ctx)
}

func (s *Service) UpdateContract(ctx context.Context, cmd UpdateContractCommand) (ContractDocument, error) {
	actor := strings.TrimSpace(cmd.Actor)
	title := strings.TrimSpace(cmd.Title)
	if cmd.ContractID <= 0 {
		return ContractDocument{}, fmt.Errorf("invalid contract id")
	}
	if title == "" {
		return ContractDocument{}, fmt.Errorf("contract title required")
	}
	return s.repo.UpdateContract(ctx, UpdateContractRecord{
		Actor:      actor,
		ContractID: cmd.ContractID,
		Title:      title,
		Note:       strings.TrimSpace(cmd.Note),
	})
}

func (s *Service) DeleteContract(ctx context.Context, cmd DeleteContractCommand) error {
	actor := strings.TrimSpace(cmd.Actor)
	if cmd.ContractID <= 0 {
		return fmt.Errorf("invalid contract id")
	}
	return s.repo.DeleteContract(ctx, DeleteContractRecord{Actor: actor, ContractID: cmd.ContractID})
}

func (s *Service) LoadContractPDFFile(ctx context.Context, contractID int64) (ContractFile, error) {
	if contractID <= 0 {
		return ContractFile{}, fmt.Errorf("invalid contract id")
	}
	return s.repo.LoadContractPDFFile(ctx, contractID)
}

func (s *Service) LoadStampedPDFFile(ctx context.Context, contractID, versionID int64, latest bool) (ContractFile, error) {
	if contractID <= 0 {
		return ContractFile{}, fmt.Errorf("invalid contract id")
	}
	if !latest && versionID <= 0 {
		return ContractFile{}, fmt.Errorf("invalid version id")
	}
	return s.repo.LoadStampedPDFFile(ctx, contractID, versionID, latest)
}

func (s *Service) UploadContract(ctx context.Context, cmd UploadContractCommand) (ContractDocument, error) {
	actor := strings.TrimSpace(cmd.Actor)
	filename := safeContractFilename(cmd.Filename)
	if len(cmd.Data) == 0 {
		return ContractDocument{}, fmt.Errorf("file required")
	}
	if len(cmd.Data) > maxContractUploadBytes {
		return ContractDocument{}, fmt.Errorf("contract file too large")
	}
	sourceKind := detectContractSourceKind(filename, cmd.ContentType, cmd.Data)
	if sourceKind == "" {
		return ContractDocument{}, fmt.Errorf("pdf or docx required")
	}
	if sourceKind == ContractSourcePDF && !looksLikePDF(cmd.Data) {
		return ContractDocument{}, fmt.Errorf("pdf file invalid")
	}

	now := s.clock()
	sourceObjectKey := contractObjectKey("contracts/source", now, filename)
	if err := writeContractAssetFile(s.assetDir, sourceObjectKey, cmd.Data); err != nil {
		return ContractDocument{}, err
	}
	written := []string{sourceObjectKey}
	committed := false
	defer func() {
		if !committed {
			for _, key := range written {
				cleanupContractAssetFile(s.assetDir, key)
			}
		}
	}()

	pdfObjectKey := sourceObjectKey
	pdfBytes := cmd.Data
	if sourceKind == ContractSourceDOCX {
		if s.converter == nil {
			return ContractDocument{}, fmt.Errorf("docx converter not configured")
		}
		sourcePath := filepath.Join(s.assetDir, sourceObjectKey)
		tmpParent := filepath.Join(s.assetDir, "contracts", "tmp")
		if err := os.MkdirAll(tmpParent, 0755); err != nil {
			return ContractDocument{}, err
		}
		tmpDir, err := os.MkdirTemp(tmpParent, "docx-*")
		if err != nil {
			return ContractDocument{}, err
		}
		defer os.RemoveAll(tmpDir)
		convertedPath, err := s.converter.ConvertDOCXToPDF(ctx, sourcePath, tmpDir)
		if err != nil {
			return ContractDocument{}, fmt.Errorf("docx convert failed: %w", err)
		}
		pdfBytes, err = os.ReadFile(convertedPath)
		if err != nil {
			return ContractDocument{}, fmt.Errorf("read converted pdf: %w", err)
		}
		if !looksLikePDF(pdfBytes) {
			return ContractDocument{}, fmt.Errorf("converted file is not pdf")
		}
		pdfObjectKey = contractObjectKey("contracts/pdf", now, strings.TrimSuffix(filename, filepath.Ext(filename))+".pdf")
		if err := writeContractAssetFile(s.assetDir, pdfObjectKey, pdfBytes); err != nil {
			return ContractDocument{}, err
		}
		written = append(written, pdfObjectKey)
	}

	sourceSum := sha256.Sum256(cmd.Data)
	pdfSum := sha256.Sum256(pdfBytes)
	doc, err := s.repo.CreateContract(ctx, CreateContractRecord{
		Actor:             actor,
		Title:             contractTitleFromFilename(filename),
		SourceFilename:    filename,
		SourceContentType: normalizeContractContentType(sourceKind, cmd.ContentType),
		SourceKind:        sourceKind,
		SourceObjectKey:   sourceObjectKey,
		SourceBytes:       int64(len(cmd.Data)),
		SourceSHA256:      hex.EncodeToString(sourceSum[:]),
		PDFObjectKey:      pdfObjectKey,
		PDFBytes:          int64(len(pdfBytes)),
		PDFSHA256:         hex.EncodeToString(pdfSum[:]),
	})
	if err != nil {
		return ContractDocument{}, err
	}
	committed = true
	return doc, nil
}

func (s *Service) SaveStampedPDF(ctx context.Context, cmd SaveStampedPDFCommand) (ContractStampedVersion, error) {
	actor := strings.TrimSpace(cmd.Actor)
	if cmd.ContractID <= 0 {
		return ContractStampedVersion{}, fmt.Errorf("invalid contract id")
	}
	if cmd.SealAssetID <= 0 {
		return ContractStampedVersion{}, fmt.Errorf("seal required")
	}
	if !looksLikePDF(cmd.Data) {
		return ContractStampedVersion{}, fmt.Errorf("stamped pdf required")
	}
	if len(cmd.Data) > maxContractUploadBytes {
		return ContractStampedVersion{}, fmt.Errorf("stamped pdf too large")
	}
	if len(cmd.Placements) == 0 {
		return ContractStampedVersion{}, fmt.Errorf("stamp placement required")
	}
	for _, p := range cmd.Placements {
		if p.PageNumber <= 0 || p.Width <= 0 || p.Height <= 0 || p.X < 0 || p.Y < 0 {
			return ContractStampedVersion{}, fmt.Errorf("invalid stamp placement")
		}
	}

	objectKey := contractObjectKey(filepath.ToSlash(filepath.Join("contracts", "stamped", fmt.Sprintf("%d", cmd.ContractID))), s.clock(), "stamped.pdf")
	if err := writeContractAssetFile(s.assetDir, objectKey, cmd.Data); err != nil {
		return ContractStampedVersion{}, err
	}
	committed := false
	defer func() {
		if !committed {
			cleanupContractAssetFile(s.assetDir, objectKey)
		}
	}()
	sum := sha256.Sum256(cmd.Data)
	version, err := s.repo.SaveStampedVersion(ctx, SaveStampedVersionRecord{
		Actor:       actor,
		ContractID:  cmd.ContractID,
		SealAssetID: cmd.SealAssetID,
		Placements:  append([]StampPlacement(nil), cmd.Placements...),
		ObjectKey:   objectKey,
		Bytes:       int64(len(cmd.Data)),
		SHA256:      hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return ContractStampedVersion{}, err
	}
	committed = true
	return version, nil
}

func detectContractSourceKind(filename, contentType string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if looksLikePDF(data) || ext == ".pdf" || ct == "application/pdf" {
		return ContractSourcePDF
	}
	if ext == ".docx" || strings.Contains(ct, "wordprocessingml.document") {
		return ContractSourceDOCX
	}
	return ""
}

func looksLikePDF(data []byte) bool {
	return len(data) >= 5 && string(data[:5]) == "%PDF-"
}

func normalizeContractContentType(kind, contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}
	if kind == ContractSourceDOCX {
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	return "application/pdf"
}

func safeContractFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "" {
		return "contract.pdf"
	}
	filename = strings.NewReplacer("/", "-", "\\", "-", ":", "-", "\x00", "").Replace(filename)
	return filename
}

func contractTitleFromFilename(filename string) string {
	base := strings.TrimSuffix(safeContractFilename(filename), filepath.Ext(filename))
	base = strings.TrimSpace(base)
	if base == "" {
		return "合同"
	}
	return base
}

func contractObjectKey(prefix string, now time.Time, filename string) string {
	name := safeContractFilename(filename)
	return filepath.ToSlash(filepath.Join(prefix, fmt.Sprintf("%d-%s", now.UnixNano(), name)))
}

func writeContractAssetFile(assetDir, objectKey string, data []byte) error {
	path := filepath.Join(assetDir, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func cleanupContractAssetFile(assetDir, objectKey string) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(objectKey)))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return
	}
	assetDir = filepath.Clean(assetDir)
	path := filepath.Clean(filepath.Join(assetDir, clean))
	if err := os.Remove(path); err != nil {
		return
	}
	for dir := filepath.Dir(path); dir != "." && dir != assetDir; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}
