package contracts

import (
	"context"
	"encoding/json"
	"fmt"
	contractsapp "orderapp/internal/application/contracts"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool     *pgxpool.Pool
	schema   string
	assetDir string
}

type Option func(*Repository)

func WithAssetDir(assetDir string) Option {
	return func(r *Repository) {
		if strings.TrimSpace(assetDir) != "" {
			r.assetDir = strings.TrimSpace(assetDir)
		}
	}
}

func NewRepository(pool *pgxpool.Pool, schema string, opts ...Option) Repository {
	r := Repository{pool: pool, schema: schema, assetDir: "/app/data/assets"}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

func (r Repository) CreateContract(ctx context.Context, record contractsapp.CreateContractRecord) (contractsapp.ContractDocument, error) {
	var doc contractsapp.ContractDocument
	q := fmt.Sprintf(`INSERT INTO %s.contract_documents(
			title, source_filename, source_content_type, source_kind, source_object_key, source_bytes, source_sha256,
			pdf_object_key, pdf_bytes, pdf_sha256, created_by
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, title, note, source_filename, source_content_type, source_kind, source_object_key, pdf_object_key, pdf_bytes,
			to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by, COALESCE(to_char(deleted_at,'YYYY-MM-DD HH24:MI:SS'), ''), deleted_by`, r.schema)
	if err := r.pool.QueryRow(ctx, q,
		record.Title,
		record.SourceFilename,
		record.SourceContentType,
		record.SourceKind,
		record.SourceObjectKey,
		record.SourceBytes,
		record.SourceSHA256,
		record.PDFObjectKey,
		record.PDFBytes,
		record.PDFSHA256,
		record.Actor,
	).Scan(&doc.ID, &doc.Title, &doc.Note, &doc.SourceFilename, &doc.SourceContentType, &doc.SourceKind, &doc.SourceObjectKey, &doc.PDFObjectKey, &doc.PDFBytes, &doc.CreatedAt, &doc.CreatedBy, &doc.DeletedAt, &doc.DeletedBy); err != nil {
		return contractsapp.ContractDocument{}, err
	}
	doc.PDFURL = contractPDFDownloadURL(doc.ID)
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, record.Actor, "contract_document", &doc.ID, "create", postgresinfra.StrPtr("source_kind"), nil, postgresinfra.StrPtr(record.SourceKind), postgresinfra.AuditMeta{"title": doc.Title, "pdf_bytes": doc.PDFBytes})
	return doc, nil
}

func (r Repository) UpdateContract(ctx context.Context, record contractsapp.UpdateContractRecord) (contractsapp.ContractDocument, error) {
	q := fmt.Sprintf(`UPDATE %s.contract_documents
		SET title=$2, note=$3
		WHERE id=$1 AND deleted_at IS NULL
		RETURNING id, title, note, source_filename, source_content_type, source_kind, source_object_key, pdf_object_key, pdf_bytes,
			to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by, COALESCE(to_char(deleted_at,'YYYY-MM-DD HH24:MI:SS'), ''), deleted_by`, r.schema)
	var doc contractsapp.ContractDocument
	if err := r.pool.QueryRow(ctx, q, record.ContractID, record.Title, record.Note).
		Scan(&doc.ID, &doc.Title, &doc.Note, &doc.SourceFilename, &doc.SourceContentType, &doc.SourceKind, &doc.SourceObjectKey, &doc.PDFObjectKey, &doc.PDFBytes, &doc.CreatedAt, &doc.CreatedBy, &doc.DeletedAt, &doc.DeletedBy); err != nil {
		return contractsapp.ContractDocument{}, err
	}
	doc.PDFURL = contractPDFDownloadURL(doc.ID)
	latest, err := r.loadLatestStampedVersion(ctx, doc.ID)
	if err != nil {
		return contractsapp.ContractDocument{}, err
	}
	if latest.ID > 0 {
		doc.LatestStamped = &latest
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, record.Actor, "contract_document", &doc.ID, "update", postgresinfra.StrPtr("title"), nil, postgresinfra.StrPtr(record.Title), postgresinfra.AuditMeta{"note": record.Note})
	return doc, nil
}

func (r Repository) DeleteContract(ctx context.Context, record contractsapp.DeleteContractRecord) error {
	tag, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.contract_documents SET deleted_at=now(), deleted_by=$2 WHERE id=$1 AND deleted_at IS NULL`, r.schema), record.ContractID, record.Actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, record.Actor, "contract_document", &record.ContractID, "delete", postgresinfra.StrPtr("deleted_at"), nil, postgresinfra.StrPtr("now"), postgresinfra.AuditMeta{})
	return nil
}

func (r Repository) SaveStampedVersion(ctx context.Context, record contractsapp.SaveStampedVersionRecord) (contractsapp.ContractStampedVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := loadStampedVersionsTx(ctx, tx, r.schema, record.ContractID, true)
	if err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	versionNo := nextStampedVersion(existing)
	placementsJSON, err := json.Marshal(record.Placements)
	if err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.contract_stamped_versions SET is_latest=false WHERE contract_id=$1`, r.schema), record.ContractID); err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	var version contractsapp.ContractStampedVersion
	var raw []byte
	q := fmt.Sprintf(`INSERT INTO %s.contract_stamped_versions(
			contract_id, version_no, seal_asset_id, placements_json, object_key, bytes, sha256, is_latest, created_by
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,true,$8)
		RETURNING id, contract_id, version_no, seal_asset_id, placements_json, object_key, bytes, is_latest,
			to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema)
	if err := tx.QueryRow(ctx, q, record.ContractID, versionNo, record.SealAssetID, placementsJSON, record.ObjectKey, record.Bytes, record.SHA256, record.Actor).
		Scan(&version.ID, &version.ContractID, &version.VersionNo, &version.SealAssetID, &raw, &version.ObjectKey, &version.Bytes, &version.IsLatest, &version.CreatedAt, &version.CreatedBy); err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	_ = json.Unmarshal(raw, &version.Placements)
	version.DownloadURL = stampedPDFDownloadURL(version.ContractID, version.ID)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, record.Actor, "contract_stamped_version", &version.ID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", version.VersionNo)), postgresinfra.AuditMeta{"contract_id": record.ContractID, "seal_asset_id": record.SealAssetID}); err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	return version, nil
}

func (r Repository) ListContracts(ctx context.Context) ([]contractsapp.ContractDocument, error) {
	q := fmt.Sprintf(`SELECT id, title, note, source_filename, source_content_type, source_kind, source_object_key,
			pdf_object_key, pdf_bytes, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by, COALESCE(to_char(deleted_at,'YYYY-MM-DD HH24:MI:SS'), ''), deleted_by
		FROM %s.contract_documents
		WHERE deleted_at IS NULL
		ORDER BY id DESC`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]contractsapp.ContractDocument, 0)
	for rows.Next() {
		var doc contractsapp.ContractDocument
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.Note, &doc.SourceFilename, &doc.SourceContentType, &doc.SourceKind, &doc.SourceObjectKey, &doc.PDFObjectKey, &doc.PDFBytes, &doc.CreatedAt, &doc.CreatedBy, &doc.DeletedAt, &doc.DeletedBy); err != nil {
			return nil, err
		}
		doc.PDFURL = contractPDFDownloadURL(doc.ID)
		latest, err := r.loadLatestStampedVersion(ctx, doc.ID)
		if err != nil {
			return nil, err
		}
		if latest.ID > 0 {
			doc.LatestStamped = &latest
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (r Repository) LoadContractPDFFile(ctx context.Context, contractID int64) (contractsapp.ContractFile, error) {
	var title, sourceFilename, objectKey string
	q := fmt.Sprintf(`SELECT title, source_filename, pdf_object_key FROM %s.contract_documents WHERE id=$1 AND deleted_at IS NULL`, r.schema)
	if err := r.pool.QueryRow(ctx, q, contractID).Scan(&title, &sourceFilename, &objectKey); err != nil {
		return contractsapp.ContractFile{}, err
	}
	filename := sourceFilename
	if strings.ToLower(filepath.Ext(filename)) != ".pdf" {
		filename = safeDownloadPart(title) + ".pdf"
	}
	return contractsapp.ContractFile{
		Path:        filepath.Join(r.assetDir, filepath.FromSlash(objectKey)),
		Filename:    filename,
		ContentType: "application/pdf",
	}, nil
}

func (r Repository) LoadStampedPDFFile(ctx context.Context, contractID, versionID int64, latest bool) (contractsapp.ContractFile, error) {
	where := "v.contract_id=$1 AND v.id=$2"
	args := []any{contractID, versionID}
	if latest {
		where = "v.contract_id=$1 AND v.is_latest=true"
		args = []any{contractID}
	}
	q := fmt.Sprintf(`SELECT d.title, v.version_no, v.object_key
		FROM %s.contract_stamped_versions v
		JOIN %s.contract_documents d ON d.id=v.contract_id
		WHERE d.deleted_at IS NULL AND %s
		ORDER BY v.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var title, objectKey string
	var versionNo int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&title, &versionNo, &objectKey); err != nil {
		return contractsapp.ContractFile{}, err
	}
	return contractsapp.ContractFile{
		Path:        filepath.Join(r.assetDir, filepath.FromSlash(objectKey)),
		Filename:    fmt.Sprintf("%s-stamped-V%d.pdf", safeDownloadPart(title), versionNo),
		ContentType: "application/pdf",
	}, nil
}

func (r Repository) loadLatestStampedVersion(ctx context.Context, contractID int64) (contractsapp.ContractStampedVersion, error) {
	q := fmt.Sprintf(`SELECT id, contract_id, version_no, seal_asset_id, placements_json, object_key, bytes, is_latest,
			to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by
		FROM %s.contract_stamped_versions
		WHERE contract_id=$1 AND is_latest=true
		ORDER BY version_no DESC
		LIMIT 1`, r.schema)
	var version contractsapp.ContractStampedVersion
	var raw []byte
	err := r.pool.QueryRow(ctx, q, contractID).Scan(&version.ID, &version.ContractID, &version.VersionNo, &version.SealAssetID, &raw, &version.ObjectKey, &version.Bytes, &version.IsLatest, &version.CreatedAt, &version.CreatedBy)
	if err == pgx.ErrNoRows {
		return contractsapp.ContractStampedVersion{}, nil
	}
	if err != nil {
		return contractsapp.ContractStampedVersion{}, err
	}
	_ = json.Unmarshal(raw, &version.Placements)
	version.DownloadURL = stampedPDFDownloadURL(version.ContractID, version.ID)
	return version, nil
}

func loadStampedVersionsTx(ctx context.Context, tx pgx.Tx, schema string, contractID int64, lock bool) ([]int, error) {
	q := fmt.Sprintf(`SELECT version_no FROM %s.contract_stamped_versions WHERE contract_id=$1`, schema)
	if lock {
		q += " FOR UPDATE"
	}
	rows, err := tx.Query(ctx, q, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make([]int, 0)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func nextStampedVersion(existing []int) int {
	maxVersion := 0
	for _, version := range existing {
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion + 1
}

func contractPDFDownloadURL(contractID int64) string {
	return fmt.Sprintf("/contracts/%d/pdf", contractID)
}

func stampedPDFDownloadURL(contractID, versionID int64) string {
	return fmt.Sprintf("/contracts/%d/stamped/%d.pdf", contractID, versionID)
}

func safeDownloadPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "contract"
	}
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(s)
}
