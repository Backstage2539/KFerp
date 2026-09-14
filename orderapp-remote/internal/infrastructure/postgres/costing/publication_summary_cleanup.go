package costing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	appcosting "orderapp/internal/application/costing"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

type publicationSummaryScanner interface{ Scan(...any) error }

func syncBeanListPublicationSummaryMetadata(ctx context.Context, tx pgx.Tx, schema string, id int64, config, content map[string]any) error {
	meta := appcosting.BeanListBatchMetadata(config)
	hasContent := appcosting.BeanListPublicationHasContent(content)
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bean_list_publications
		SET publication_release_id=$2,publication_table_key=$3,publication_table_name=$4,
		    publication_is_default_table=$5,publication_has_content=$6,publication_summary_ready=true
		WHERE id=$1`, schema), id, meta.ReleaseID, meta.TableKey, meta.TableName, meta.IsDefaultTable, hasContent)
	return err
}

const publicationSummaryColumns = `
	id, COALESCE(NULLIF(publication_purpose,''),'factory_supply'), list_type,
	COALESCE(product_type_category_id,0), COALESCE(product_type_name,''),
	COALESCE(classification_template_id,0), COALESCE(classification_template_name,''),
	COALESCE(classification_category_id,0), COALESCE(classification_category_name,''),
	version_no, status, owner_type, owner_key,
	COALESCE(price_source_publication_id,0), COALESCE(style_source_publication_id,0), COALESCE(source_version_no,''),
	COALESCE(NULLIF(publication_release_id,''),'legacy:' || id::text),
	COALESCE(NULLIF(publication_table_key,''),id::text),
	COALESCE(NULLIF(publication_table_name,''),version_no), publication_is_default_table,
	publication_has_content, changelog,
	to_char(published_at,'YYYY-MM-DD HH24:MI'), COALESCE(to_char(withdrawn_at,'YYYY-MM-DD HH24:MI'),''), to_char(created_at,'YYYY-MM-DD HH24:MI')`

func scanPublicationSummary(row publicationSummaryScanner) (appcosting.BeanListPublicationSummary, error) {
	var out appcosting.BeanListPublicationSummary
	err := row.Scan(&out.ID, &out.PublicationPurpose, &out.ListType,
		&out.ProductTypeCategoryID, &out.ProductTypeName, &out.ClassificationTemplateID, &out.ClassificationTemplateName,
		&out.ClassificationCategoryID, &out.ClassificationCategoryName, &out.Version, &out.Status, &out.OwnerType, &out.OwnerKey,
		&out.PriceSourcePublicationID, &out.StyleSourcePublicationID, &out.SourceVersion,
		&out.ReleaseID, &out.TableKey, &out.TableName, &out.IsDefaultTable, &out.HasContent, &out.Changelog,
		&out.PublishedAt, &out.WithdrawnAt, &out.CreatedAt)
	return out, err
}

func publicationSummaryBaseScope(query appcosting.BeanListPublicationQuery, includeDeleted bool) (string, []any) {
	args := []any{strings.TrimSpace(query.PublicationPurpose), strings.TrimSpace(query.OwnerType), strings.TrimSpace(query.OwnerKey), query.ClassificationTemplateID, strings.TrimSpace(query.ListType)}
	where := "publication_purpose=$1 AND owner_type=$2 AND owner_key=$3"
	if !includeDeleted {
		where += " AND deleted_at IS NULL AND status<>'deleted'"
	}
	if query.ClassificationTemplateID > 0 {
		where += " AND (COALESCE(classification_template_id,0)=$4 OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=$4) OR (COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=0 AND list_type=$5))"
	} else if query.ProductTypeCategoryID > 0 {
		args[3] = query.ProductTypeCategoryID
		where += " AND (COALESCE(product_type_category_id,0)=$4 OR (COALESCE(product_type_category_id,0)=0 AND list_type=$5))"
	} else {
		where += " AND list_type=$5 AND $4::bigint=0"
	}
	return where, args
}

func publicationSummaryScope(query appcosting.BeanListPublicationQuery, status string) (string, []any) {
	where, args := publicationSummaryBaseScope(query, false)
	if status == "archived" {
		where += " AND status='archived'"
	} else if status == "published" {
		where += " AND status='published'"
	} else {
		where += " AND status<>'archived'"
	}
	return where, args
}

func publicationSummaryPreferenceOrder(query appcosting.BeanListPublicationQuery) string {
	if query.ClassificationTemplateID > 0 {
		return "CASE WHEN COALESCE(classification_template_id,0)=$4 THEN 0 WHEN COALESCE(classification_template_id,0)=0 AND COALESCE(product_type_category_id,0)=$4 THEN 1 ELSE 2 END,"
	}
	if query.ProductTypeCategoryID > 0 {
		return "CASE WHEN COALESCE(product_type_category_id,0)=$4 THEN 0 ELSE 1 END,"
	}
	return ""
}

func (r Repository) ListBeanListPublicationSummaries(ctx context.Context, query appcosting.BeanListPublicationSummaryQuery) (appcosting.BeanListPublicationSummaryPage, error) {
	where, args := publicationSummaryScope(query.BeanListPublicationQuery, query.Status)
	args = append(args, "%"+strings.ToLower(query.Search)+"%", query.PageSize, (query.Page-1)*query.PageSize)
	batchExpr := "COALESCE(NULLIF(publication_release_id,''),'legacy:' || id::text)"
	sql := fmt.Sprintf(`WITH scoped AS (
		SELECT *, %[1]s AS batch_key FROM %[2]s.bean_list_publications WHERE %[3]s
	), matching AS (
		SELECT batch_key, max(created_at) AS batch_created, max(id) AS batch_id
		FROM scoped WHERE $6='%%' OR lower(concat_ws(' ',version_no,publication_table_name,owner_key,status,changelog,product_type_name,classification_template_name)) LIKE $6
		GROUP BY batch_key
	), page_keys AS (
		SELECT batch_key,batch_created,batch_id FROM matching ORDER BY batch_created DESC,batch_id DESC LIMIT $7 OFFSET $8
	)
	SELECT %[4]s FROM scoped s JOIN page_keys p USING(batch_key)
	ORDER BY p.batch_created DESC,p.batch_id DESC,s.publication_is_default_table DESC,s.id DESC`, batchExpr, r.schema, where, publicationSummaryColumns)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return appcosting.BeanListPublicationSummaryPage{}, err
	}
	defer rows.Close()
	out := appcosting.BeanListPublicationSummaryPage{Rows: []appcosting.BeanListPublicationSummary{}, Page: query.Page, PageSize: query.PageSize}
	for rows.Next() {
		row, err := scanPublicationSummary(rows)
		if err != nil {
			return out, err
		}
		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	countSQL := fmt.Sprintf(`SELECT count(DISTINCT %s) FROM %s.bean_list_publications WHERE %s AND ($6='%%' OR lower(concat_ws(' ',version_no,publication_table_name,owner_key,status,changelog,product_type_name,classification_template_name)) LIKE $6)`, batchExpr, r.schema, where)
	if err := r.pool.QueryRow(ctx, countSQL, args[:6]...).Scan(&out.Total); err != nil {
		return out, err
	}
	archivedWhere, archivedArgs := publicationSummaryScope(query.BeanListPublicationQuery, "archived")
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.bean_list_publications WHERE %s`, r.schema, archivedWhere), archivedArgs...).Scan(&out.ArchivedTotal); err != nil {
		return out, err
	}
	publishedWhere, publishedArgs := publicationSummaryScope(query.BeanListPublicationQuery, "published")
	current, err := scanPublicationSummary(r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s.bean_list_publications WHERE %s ORDER BY %s published_at DESC,publication_is_default_table DESC,id DESC LIMIT 1`, publicationSummaryColumns, r.schema, publishedWhere, publicationSummaryPreferenceOrder(query.BeanListPublicationQuery)), publishedArgs...))
	if err == nil {
		out.Current = &current
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	versionWhere, versionArgs := publicationSummaryBaseScope(query.BeanListPublicationQuery, true)
	versionRows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT version_no FROM %s.bean_list_publications WHERE %s AND status<>'draft'`, r.schema, versionWhere), versionArgs...)
	if err != nil {
		return out, err
	}
	versions := []appcosting.BeanListPublication{}
	for versionRows.Next() {
		var version string
		if err := versionRows.Scan(&version); err != nil {
			versionRows.Close()
			return out, err
		}
		versions = append(versions, appcosting.BeanListPublication{Version: version})
	}
	versionRows.Close()
	out.SuggestedVersion = appcosting.NextBeanListPublicationVersion("", versions)
	return out, nil
}

func (r Repository) ListBeanListPriceSources(ctx context.Context, query appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	ids := []int64{}
	for _, candidate := range []struct{ ownerType, ownerKey, statuses string }{
		{"customer", query.OwnerKey, "status IN ('draft','published')"},
		{"official", "", "status='published'"},
	} {
		q := query
		q.OwnerType, q.OwnerKey = candidate.ownerType, candidate.ownerKey
		where, args := publicationSummaryScope(q, "active")
		where = strings.ReplaceAll(where, "status<>'archived'", candidate.statuses)
		var id int64
		err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.bean_list_publications WHERE %s ORDER BY CASE WHEN status='draft' THEN 0 ELSE 1 END,%s created_at DESC,publication_is_default_table DESC,id DESC LIMIT 1`, r.schema, where, publicationSummaryPreferenceOrder(q)), args...).Scan(&id)
		if err == nil {
			ids = append(ids, id)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	result := []appcosting.BeanListPublication{}
	for _, id := range ids {
		var ownerType, ownerKey string
		if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT owner_type,owner_key FROM %s.bean_list_publications WHERE id=$1`, r.schema), id).Scan(&ownerType, &ownerKey); err != nil {
			return nil, err
		}
		q := query
		q.OwnerType, q.OwnerKey = ownerType, ownerKey
		row, err := r.LoadBeanListPublication(ctx, q, id)
		if err != nil {
			return nil, err
		}
		if row != nil {
			result = append(result, *row)
		}
	}
	return result, nil
}

func (r Repository) ListBeanListPublicationVersions(ctx context.Context, query appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error) {
	where, args := publicationSummaryBaseScope(query, true)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT version_no FROM %s.bean_list_publications WHERE %s AND status<>'draft'`, r.schema, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []appcosting.BeanListPublication{}
	for rows.Next() {
		var row appcosting.BeanListPublication
		if err := rows.Scan(&row.Version); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func randomConfirmationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func confirmationTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (r Repository) PreviewDeleteBeanListPublications(ctx context.Context, cmd appcosting.DeleteBeanListPublicationsPreviewCommand) (appcosting.DeleteBeanListPublicationsPreview, error) {
	where, args := publicationSummaryScope(cmd.Query, "archived")
	if !cmd.ClearAll {
		args = append(args, cmd.IDs)
		where += fmt.Sprintf(" AND id=ANY($%d)", len(args))
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT %s,updated_at::text FROM %s.bean_list_publications WHERE %s ORDER BY created_at DESC,id DESC`, publicationSummaryColumns, r.schema, where), args...)
	if err != nil {
		return appcosting.DeleteBeanListPublicationsPreview{}, err
	}
	defer rows.Close()
	result := appcosting.DeleteBeanListPublicationsPreview{Rows: []appcosting.BeanListPublicationSummary{}}
	versions := map[string]string{}
	for rows.Next() {
		var row appcosting.BeanListPublicationSummary
		var updated string
		err := rows.Scan(&row.ID, &row.PublicationPurpose, &row.ListType, &row.ProductTypeCategoryID, &row.ProductTypeName, &row.ClassificationTemplateID, &row.ClassificationTemplateName, &row.ClassificationCategoryID, &row.ClassificationCategoryName, &row.Version, &row.Status, &row.OwnerType, &row.OwnerKey, &row.PriceSourcePublicationID, &row.StyleSourcePublicationID, &row.SourceVersion, &row.ReleaseID, &row.TableKey, &row.TableName, &row.IsDefaultTable, &row.HasContent, &row.Changelog, &row.PublishedAt, &row.WithdrawnAt, &row.CreatedAt, &updated)
		if err != nil {
			return result, err
		}
		result.Rows = append(result.Rows, row)
		versions[strconv.FormatInt(row.ID, 10)] = updated
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if len(result.Rows) == 0 || (!cmd.ClearAll && len(result.Rows) != len(cmd.IDs)) {
		return result, fmt.Errorf("all selected publications must be archived in the current scope")
	}
	token, err := randomConfirmationToken()
	if err != nil {
		return result, err
	}
	expires := time.Now().Add(10 * time.Minute)
	ids := make([]int64, len(result.Rows))
	for i, row := range result.Rows {
		ids[i] = row.ID
	}
	raw, _ := json.Marshal(versions)
	_, err = r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publication_delete_previews(token_hash,actor,publication_ids,row_versions,expires_at) VALUES($1,$2,$3,$4,$5)`, r.schema), confirmationTokenHash(token), cmd.Actor, ids, raw, expires)
	if err != nil {
		return result, err
	}
	result.ConfirmationToken = token
	result.ExpiresAt = expires.Format(time.RFC3339)
	result.Count = len(ids)
	return result, nil
}

func (r Repository) DeleteBeanListPublications(ctx context.Context, cmd appcosting.DeleteBeanListPublicationsCommand) (appcosting.DeleteBeanListPublicationsResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return appcosting.DeleteBeanListPublicationsResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var ids []int64
	var rawVersions, rawResult []byte
	var expires time.Time
	var completed *time.Time
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT publication_ids,row_versions,expires_at,completed_at,result_json FROM %s.bean_list_publication_delete_previews WHERE token_hash=$1 AND actor=$2 FOR UPDATE`, r.schema), confirmationTokenHash(cmd.ConfirmationToken), cmd.Actor).Scan(&ids, &rawVersions, &expires, &completed, &rawResult)
	if errors.Is(err, pgx.ErrNoRows) {
		return appcosting.DeleteBeanListPublicationsResult{}, fmt.Errorf("invalid deletion confirmation")
	}
	if err != nil {
		return appcosting.DeleteBeanListPublicationsResult{}, err
	}
	if completed != nil {
		var out appcosting.DeleteBeanListPublicationsResult
		if err := json.Unmarshal(rawResult, &out); err != nil {
			return out, err
		}
		_ = tx.Commit(ctx)
		return out, nil
	}
	if time.Now().After(expires) {
		return appcosting.DeleteBeanListPublicationsResult{}, fmt.Errorf("deletion confirmation expired")
	}
	versions := map[string]string{}
	if err := json.Unmarshal(rawVersions, &versions); err != nil {
		return appcosting.DeleteBeanListPublicationsResult{}, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,updated_at::text,publication_purpose,list_type,version_no,owner_type,owner_key,publication_release_id,publication_table_key,publication_table_name FROM %s.bean_list_publications WHERE id=ANY($1) AND status='archived' AND deleted_at IS NULL FOR UPDATE`, r.schema), ids)
	if err != nil {
		return appcosting.DeleteBeanListPublicationsResult{}, err
	}
	type doomed struct {
		id                                                                                       int64
		updated, purpose, listType, version, ownerType, ownerKey, releaseID, tableKey, tableName string
	}
	doomedRows := []doomed{}
	for rows.Next() {
		var row doomed
		if err := rows.Scan(&row.id, &row.updated, &row.purpose, &row.listType, &row.version, &row.ownerType, &row.ownerKey, &row.releaseID, &row.tableKey, &row.tableName); err != nil {
			rows.Close()
			return appcosting.DeleteBeanListPublicationsResult{}, err
		}
		if versions[strconv.FormatInt(row.id, 10)] != row.updated {
			rows.Close()
			return appcosting.DeleteBeanListPublicationsResult{}, fmt.Errorf("archived publications changed; preview again")
		}
		doomedRows = append(doomedRows, row)
	}
	rows.Close()
	if len(doomedRows) != len(ids) {
		return appcosting.DeleteBeanListPublicationsResult{}, fmt.Errorf("archived publications changed; preview again")
	}
	if err := r.rejectBoundCustomerOrderPriceTablePublications(ctx, tx, ids); err != nil {
		return appcosting.DeleteBeanListPublicationsResult{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.bean_list_publication_assets WHERE publication_id=ANY($1)`, r.schema), ids); err != nil {
		return appcosting.DeleteBeanListPublicationsResult{}, err
	}
	for _, row := range doomedRows {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bean_list_publications SET status='deleted',config_json='{}'::jsonb,content_json='{}'::jsonb,publication_has_content=false,price_source_publication_id=NULL,style_source_publication_id=NULL,source_version_no='',deleted_at=now(),deleted_by=$2,updated_at=now() WHERE id=$1`, r.schema), row.id, cmd.Actor); err != nil {
			return appcosting.DeleteBeanListPublicationsResult{}, err
		}
		id := row.id
		if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &id, "delete_archived", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("archived"), postgresinfra.StrPtr("deleted"), postgresinfra.AuditMeta{"publication_purpose": row.purpose, "list_type": row.listType, "version": row.version, "owner_type": row.ownerType, "owner_key": row.ownerKey, "release_id": row.releaseID, "table_key": row.tableKey, "table_name": row.tableName}); err != nil {
			return appcosting.DeleteBeanListPublicationsResult{}, err
		}
	}
	out := appcosting.DeleteBeanListPublicationsResult{DeletedCount: len(ids), IDs: ids}
	encoded, _ := json.Marshal(out)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bean_list_publication_delete_previews SET completed_at=now(),result_json=$2 WHERE token_hash=$1`, r.schema), confirmationTokenHash(cmd.ConfirmationToken), encoded); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}
