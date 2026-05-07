package sales

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"path/filepath"

	"github.com/jackc/pgx/v5"
)

type externalShareSource struct {
	resourceID  int64
	orderID     int64
	orderNo     string
	versionNo   int
	objectKey   string
	contentType string
	filename    string
	title       string
}

func (r Repository) CreateExternalShareResource(ctx context.Context, cmd salesapp.CreateExternalShareResourceCommand) (salesapp.ExternalShareResource, error) {
	source, err := r.resolveExternalShareSource(ctx, cmd.ResourceType, cmd.OrderID, cmd.DocumentID, cmd.Latest)
	if err != nil {
		return salesapp.ExternalShareResource{}, err
	}
	token, err := newExternalShareToken()
	if err != nil {
		return salesapp.ExternalShareResource{}, err
	}
	q := fmt.Sprintf(`INSERT INTO %s.external_share_resources(token, resource_type, order_id, resource_id, title, filename, content_type, created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING token, resource_type, order_id, resource_id, title, filename, content_type, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema)
	var out salesapp.ExternalShareResource
	if err := r.pool.QueryRow(ctx, q, token, cmd.ResourceType, source.orderID, source.resourceID, source.title, source.filename, source.contentType, cmd.Actor).Scan(
		&out.Token,
		&out.ResourceType,
		&out.OrderID,
		&out.ResourceID,
		&out.Title,
		&out.Filename,
		&out.ContentType,
		&out.CreatedAt,
		&out.CreatedBy,
	); err != nil {
		return salesapp.ExternalShareResource{}, err
	}
	fillExternalShareURLs(&out)
	return out, nil
}

func (r Repository) LoadExternalShareResourceFile(ctx context.Context, token string) (salesapp.ExternalShareResourceFile, error) {
	q := fmt.Sprintf(`SELECT token, resource_type, order_id, resource_id, title, filename, content_type, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by
		FROM %s.external_share_resources
		WHERE token=$1`, r.schema)
	var out salesapp.ExternalShareResource
	if err := r.pool.QueryRow(ctx, q, token).Scan(
		&out.Token,
		&out.ResourceType,
		&out.OrderID,
		&out.ResourceID,
		&out.Title,
		&out.Filename,
		&out.ContentType,
		&out.CreatedAt,
		&out.CreatedBy,
	); err != nil {
		return salesapp.ExternalShareResourceFile{}, err
	}
	source, err := r.resolveExternalShareSource(ctx, out.ResourceType, out.OrderID, out.ResourceID, false)
	if err != nil {
		return salesapp.ExternalShareResourceFile{}, err
	}
	_, _ = r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.external_share_resources SET last_accessed_at=now() WHERE token=$1`, r.schema), token)
	fillExternalShareURLs(&out)
	return salesapp.ExternalShareResourceFile{Resource: out, Path: filepath.Join(r.assetDir, source.objectKey)}, nil
}

func (r Repository) resolveExternalShareSource(ctx context.Context, resourceType string, orderID, documentID int64, latest bool) (externalShareSource, error) {
	switch resourceType {
	case salesapp.ExternalShareSalesOrderPDF:
		return r.resolveSalesOrderPDFShareSource(ctx, orderID, documentID, latest)
	case salesapp.ExternalShareSalesOrderImage:
		return r.resolveSalesOrderImageShareSource(ctx, orderID, documentID, latest)
	case salesapp.ExternalShareDeliveryNotePDF:
		return r.resolveDeliveryNotePDFShareSource(ctx, orderID, documentID, latest)
	default:
		return externalShareSource{}, fmt.Errorf("invalid share resource type")
	}
}

func (r Repository) resolveSalesOrderPDFShareSource(ctx context.Context, orderID, documentID int64, latest bool) (externalShareSource, error) {
	where := "d.order_id=$1 AND d.id=$2"
	args := []any{orderID, documentID}
	if latest {
		where = "d.order_id=$1 AND d.is_latest=true"
		args = []any{orderID}
	}
	q := fmt.Sprintf(`SELECT d.id, d.order_id, d.order_no, d.version_no, a.object_key
		FROM %s.sales_order_documents d
		JOIN %s.sales_order_assets a ON a.id=d.pdf_asset_id
		WHERE %s
		ORDER BY d.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var src externalShareSource
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&src.resourceID, &src.orderID, &src.orderNo, &src.versionNo, &src.objectKey); err != nil {
		if err == pgx.ErrNoRows {
			return externalShareSource{}, fmt.Errorf("sales order pdf not generated")
		}
		return externalShareSource{}, err
	}
	src.contentType = "application/pdf"
	src.filename = fmt.Sprintf("%s-V%d.pdf", safeSalesOrderPathPart(src.orderNo), src.versionNo)
	src.title = fmt.Sprintf("销售单 %s V%d", src.orderNo, src.versionNo)
	return src, nil
}

func (r Repository) resolveSalesOrderImageShareSource(ctx context.Context, orderID, imageID int64, latest bool) (externalShareSource, error) {
	where := "d.order_id=$1 AND d.id=$2"
	args := []any{orderID, imageID}
	if latest {
		where = "d.order_id=$1 AND d.is_latest=true"
		args = []any{orderID}
	}
	q := fmt.Sprintf(`SELECT d.id, d.order_id, d.order_no, d.version_no, a.object_key
		FROM %s.sales_order_images d
		JOIN %s.sales_order_assets a ON a.id=d.image_asset_id
		WHERE %s
		ORDER BY d.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var src externalShareSource
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&src.resourceID, &src.orderID, &src.orderNo, &src.versionNo, &src.objectKey); err != nil {
		if err == pgx.ErrNoRows {
			return externalShareSource{}, fmt.Errorf("sales order image not generated")
		}
		return externalShareSource{}, err
	}
	src.contentType = "image/png"
	src.filename = fmt.Sprintf("%s-V%d.png", safeSalesOrderPathPart(src.orderNo), src.versionNo)
	src.title = fmt.Sprintf("销售单图片 %s V%d", src.orderNo, src.versionNo)
	return src, nil
}

func (r Repository) resolveDeliveryNotePDFShareSource(ctx context.Context, orderID, documentID int64, latest bool) (externalShareSource, error) {
	where := "d.order_id=$1 AND d.id=$2"
	args := []any{orderID, documentID}
	if latest {
		where = "d.order_id=$1 AND d.is_latest=true"
		args = []any{orderID}
	}
	q := fmt.Sprintf(`SELECT d.id, d.order_id, d.order_no, d.version_no, a.object_key
		FROM %s.delivery_note_documents d
		JOIN %s.delivery_note_assets a ON a.id=d.pdf_asset_id
		WHERE %s
		ORDER BY d.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var src externalShareSource
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&src.resourceID, &src.orderID, &src.orderNo, &src.versionNo, &src.objectKey); err != nil {
		if err == pgx.ErrNoRows {
			return externalShareSource{}, fmt.Errorf("delivery note pdf not generated")
		}
		return externalShareSource{}, err
	}
	src.contentType = "application/pdf"
	src.filename = fmt.Sprintf("%s-DN-V%d.pdf", safeDeliveryNotePathPart(src.orderNo), src.versionNo)
	src.title = fmt.Sprintf("出库单 %s V%d", src.orderNo, src.versionNo)
	return src, nil
}

func fillExternalShareURLs(out *salesapp.ExternalShareResource) {
	out.ShareURL = "/share/" + out.Token
	out.FileURL = out.ShareURL + "/file"
	out.ShareText = out.Title + "\n" + out.ShareURL
}

func newExternalShareToken() (string, error) {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
