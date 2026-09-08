package costing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	appcosting "orderapp/internal/application/costing"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) lockBeanListVersion(ctx context.Context, tx pgx.Tx, cmd *appcosting.PublishBeanListCommand, publish bool) error {
	groupID := cmd.ClassificationTemplateID
	if groupID <= 0 {
		groupID = cmd.ProductTypeCategoryID
	}
	key := fmt.Sprintf("%s:price-table:%s:%s:%s:%s:%d", r.schema, cmd.PublicationPurpose, cmd.OwnerType, cmd.OwnerKey, cmd.ListType, groupID)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
		return err
	}
	if !publish {
		return nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT version_no FROM %s.bean_list_publications
		WHERE publication_purpose=$1 AND owner_type=$2 AND owner_key=$3 AND list_type=$4 AND status <> 'draft'
		AND COALESCE(NULLIF(classification_template_id,0),product_type_category_id,0)=$5`, r.schema), cmd.PublicationPurpose, cmd.OwnerType, cmd.OwnerKey, cmd.ListType, groupID)
	if err != nil {
		return err
	}
	versions := []appcosting.BeanListPublication{}
	for rows.Next() {
		var row appcosting.BeanListPublication
		if err := rows.Scan(&row.Version); err != nil {
			rows.Close()
			return err
		}
		versions = append(versions, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	cmd.Version = appcosting.NextBeanListPublicationVersion(cmd.Version, versions)
	return nil
}

func (r Repository) SaveBeanListBatch(ctx context.Context, commands []appcosting.PublishBeanListCommand, publish bool) ([]appcosting.BeanListPublication, error) {
	if len(commands) == 0 {
		return nil, fmt.Errorf("至少保留一张价格表")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	common := commands[0]
	if err := r.lockBeanListVersion(ctx, tx, &common, publish); err != nil {
		return nil, err
	}
	var releaseBytes [16]byte
	if _, err := rand.Read(releaseBytes[:]); err != nil {
		return nil, err
	}
	releaseID := hex.EncodeToString(releaseBytes[:])
	status := "draft"
	if publish {
		status = "published"
	}
	result := make([]appcosting.BeanListPublication, 0, len(commands))
	for _, cmd := range commands {
		cmd.Version = common.Version
		cmd.Config["version"] = common.Version
		meta := appcosting.BeanListBatchMetadata(cmd.Config)
		meta.ReleaseID = releaseID
		cmd.Content["title"] = meta.TableName + " · " + common.Version
		appcosting.SetBeanListBatchMetadata(cmd.Config, meta)
		row, err := r.insertBeanListBatchTable(ctx, tx, cmd, status)
		if err != nil {
			return nil, fmt.Errorf("价格表「%s」：%w", meta.TableName, err)
		}
		result = append(result, row)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (r Repository) insertBeanListBatchTable(ctx context.Context, tx pgx.Tx, cmd appcosting.PublishBeanListCommand, status string) (appcosting.BeanListPublication, error) {
	var row appcosting.BeanListPublication
	if err := validateBeanListProductScope(ctx, tx, r.schema, cmd); err != nil {
		return row, err
	}
	config, err := json.Marshal(cmd.Config)
	if err != nil {
		return row, err
	}
	content, err := json.Marshal(cmd.Content)
	if err != nil {
		return row, err
	}
	row = appcosting.BeanListPublication{PublicationTableMetadata: appcosting.BeanListBatchMetadata(cmd.Config), PublicationPurpose: cmd.PublicationPurpose, ListType: cmd.ListType, ProductTypeCategoryID: cmd.ProductTypeCategoryID, ProductTypeName: cmd.ProductTypeName, ClassificationTemplateID: cmd.ClassificationTemplateID, ClassificationTemplateName: cmd.ClassificationTemplateName, ClassificationCategoryID: cmd.ClassificationCategoryID, ClassificationCategoryName: cmd.ClassificationCategoryName, Version: cmd.Version, Status: status, OwnerType: cmd.OwnerType, OwnerKey: cmd.OwnerKey, PriceSourcePublicationID: cmd.PriceSourcePublicationID, StyleSourcePublicationID: cmd.StyleSourcePublicationID, SourceVersion: cmd.SourceVersion, Config: cmd.Config, Content: cmd.Content, Changelog: cmd.Changelog}
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications
	(publication_purpose,list_type,product_type_category_id,product_type_name,classification_template_id,classification_template_name,classification_category_id,classification_category_name,version_no,status,owner_type,owner_key,price_source_publication_id,style_source_publication_id,source_version_no,config_json,content_json,changelog,actor)
	VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,0),NULLIF($14,0),$15,$16::jsonb,$17::jsonb,$18,$19)
	RETURNING id,to_char(published_at,'YYYY-MM-DD HH24:MI'),to_char(created_at,'YYYY-MM-DD HH24:MI')`, r.schema), cmd.PublicationPurpose, cmd.ListType, cmd.ProductTypeCategoryID, cmd.ProductTypeName, cmd.ClassificationTemplateID, cmd.ClassificationTemplateName, cmd.ClassificationCategoryID, cmd.ClassificationCategoryName, cmd.Version, status, cmd.OwnerType, cmd.OwnerKey, cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersion, config, content, cmd.Changelog, cmd.Actor).Scan(&row.ID, &row.PublishedAt, &row.CreatedAt)
	if err != nil {
		return row, err
	}
	action := "save_draft"
	if status == "published" {
		action = "publish"
	}
	err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bean_list_publication", &row.ID, action, postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr(status), postgresinfra.AuditMeta{"release_id": row.ReleaseID, "table_key": row.TableKey, "table_name": row.TableName, "is_default_table": row.IsDefaultTable, "version": row.Version, "owner_type": row.OwnerType, "owner_key": row.OwnerKey, "publication_purpose": row.PublicationPurpose, "list_type": row.ListType, "product_type_category_id": row.ProductTypeCategoryID, "classification_template_id": row.ClassificationTemplateID})
	return row, err
}

// Expand only within the authorized owner/purpose. A legacy ID expands to itself.
func (r Repository) expandBeanListBatchIDs(ctx context.Context, tx pgx.Tx, ids []int64, purpose, ownerType, ownerKey string) ([]int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT b.id FROM %[1]s.bean_list_publications b
	WHERE b.publication_purpose=$2 AND b.owner_type=$3 AND b.owner_key=$4
	AND (b.id=ANY($1) OR NULLIF(b.config_json->'publication_batch'->>'release_id','') IN
	(SELECT NULLIF(s.config_json->'publication_batch'->>'release_id','') FROM %[1]s.bean_list_publications s WHERE s.id=ANY($1) AND s.publication_purpose=$2 AND s.owner_type=$3 AND s.owner_key=$4))
	ORDER BY b.id FOR UPDATE OF b`, r.schema), ids, purpose, ownerType, ownerKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}
