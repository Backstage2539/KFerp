package productcreator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	app "orderapp/internal/application/productcreator"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ListTemplates(ctx context.Context) ([]app.Template, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,name,description,status,revision,published_version,draft_graph,updated_at FROM %s.business_templates ORDER BY updated_at DESC,id DESC`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.Template, 0)
	for rows.Next() {
		row, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) GetTemplate(ctx context.Context, id int64) (app.Template, error) {
	row, err := scanTemplate(r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,name,description,status,revision,published_version,draft_graph,updated_at FROM %s.business_templates WHERE id=$1`, r.schema), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Template{}, app.ErrNotFound
	}
	return row, err
}

func (r Repository) SaveTemplate(ctx context.Context, input app.TemplateSave) (app.Template, error) {
	graph, err := json.Marshal(input.Workflow)
	if err != nil {
		return app.Template{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.Template{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	if input.ID == 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_templates(name,description,draft_graph,created_by,updated_by) VALUES($1,$2,$3::jsonb,$4,$4) RETURNING id`, r.schema), input.Name, input.Description, graph, input.Actor).Scan(&id); err != nil {
			return app.Template{}, err
		}
	} else {
		id = input.ID
		command, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.business_templates SET name=$3,description=$4,draft_graph=$5::jsonb,revision=revision+1,updated_by=$6,updated_at=now() WHERE id=$1 AND revision=$2 AND status<>'disabled'`, r.schema), id, input.ExpectedRev, input.Name, input.Description, graph, input.Actor)
		if err != nil {
			return app.Template{}, err
		}
		if command.RowsAffected() == 0 {
			return app.Template{}, r.resolveTemplateWriteConflict(ctx, tx, id)
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, input.Actor, "business_template", &id, "save", postgresinfra.StrPtr("workflow"), nil, postgresinfra.StrPtr("updated"), postgresinfra.AuditMeta{"template_id": id, "node_count": len(input.Workflow.Nodes), "edge_count": len(input.Workflow.Edges)}); err != nil {
		return app.Template{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.Template{}, err
	}
	return r.GetTemplate(ctx, id)
}

func (r Repository) CopyTemplate(ctx context.Context, id int64, name, actor string) (app.Template, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.Template{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var description string
	var graph []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT description,draft_graph FROM %s.business_templates WHERE id=$1 AND status<>'disabled'`, r.schema), id).Scan(&description, &graph); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.Template{}, app.ErrNotFound
		}
		return app.Template{}, err
	}
	var newID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_templates(name,description,draft_graph,created_by,updated_by) VALUES($1,$2,$3::jsonb,$4,$4) RETURNING id`, r.schema), name, description, graph, actor).Scan(&newID); err != nil {
		return app.Template{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "business_template", &newID, "copy", postgresinfra.StrPtr("source_template_id"), postgresinfra.StrPtr(fmt.Sprint(id)), postgresinfra.StrPtr(fmt.Sprint(newID)), postgresinfra.AuditMeta{"source_template_id": id, "template_id": newID}); err != nil {
		return app.Template{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.Template{}, err
	}
	return r.GetTemplate(ctx, newID)
}

func (r Repository) PublishTemplate(ctx context.Context, id, revision int64, actor string) (app.TemplateVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.TemplateVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var name, description string
	var graph []byte
	var currentRevision int64
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT name,description,draft_graph,revision,status FROM %s.business_templates WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&name, &description, &graph, &currentRevision, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.TemplateVersion{}, app.ErrNotFound
		}
		return app.TemplateVersion{}, err
	}
	if currentRevision != revision {
		return app.TemplateVersion{}, app.ErrConflict
	}
	if status == "disabled" {
		return app.TemplateVersion{}, fmt.Errorf("disabled template cannot be published")
	}
	var version int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(version),0)+1 FROM %s.business_template_versions WHERE template_id=$1`, r.schema), id).Scan(&version); err != nil {
		return app.TemplateVersion{}, err
	}
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.business_template_versions(template_id,version,name,description,workflow,published_by) VALUES($1,$2,$3,$4,$5::jsonb,$6) RETURNING id`, r.schema), id, version, name, description, graph, actor).Scan(&versionID); err != nil {
		return app.TemplateVersion{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.business_templates SET status='published',published_version=$2,revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1`, r.schema), id, version, actor); err != nil {
		return app.TemplateVersion{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "business_template", &id, "publish", postgresinfra.StrPtr("version"), nil, postgresinfra.StrPtr(fmt.Sprint(version)), postgresinfra.AuditMeta{"template_id": id, "version_id": versionID, "version": version}); err != nil {
		return app.TemplateVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.TemplateVersion{}, err
	}
	return r.getVersion(ctx, versionID)
}

func (r Repository) ListVersions(ctx context.Context, templateID int64) ([]app.TemplateVersion, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,template_id,version,name,description,workflow,published_at,published_by FROM %s.business_template_versions WHERE template_id=$1 ORDER BY version DESC`, r.schema), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.TemplateVersion, 0)
	for rows.Next() {
		row, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) DisableTemplate(ctx context.Context, id, revision int64, actor string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.business_templates SET status='disabled',revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1 AND revision=$2 AND status<>'disabled'`, r.schema), id, revision, actor)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return r.resolveTemplateWriteConflict(ctx, tx, id)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "business_template", &id, "disable", nil, nil, nil, postgresinfra.AuditMeta{"template_id": id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) StartRun(ctx context.Context, templateID int64, actor string) (app.Run, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var version int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT published_version FROM %s.business_templates WHERE id=$1 AND status='published' AND published_version>0 FOR SHARE`, r.schema), templateID).Scan(&version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.Run{}, app.ErrNotPublished
		}
		return app.Run{}, err
	}
	var workflow []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT workflow FROM %s.business_template_versions WHERE template_id=$1 AND version=$2`, r.schema), templateID, version).Scan(&workflow); err != nil {
		return app.Run{}, err
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.product_creator_runs(template_id,template_version,workflow_snapshot,created_by) VALUES($1,$2,$3::jsonb,$4) RETURNING id`, r.schema), templateID, version, workflow, actor).Scan(&id); err != nil {
		return app.Run{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_creator_run", &id, "create", postgresinfra.StrPtr("template_version"), nil, postgresinfra.StrPtr(fmt.Sprint(version)), postgresinfra.AuditMeta{"run_id": id, "template_id": templateID, "template_version": version}); err != nil {
		return app.Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.Run{}, err
	}
	return r.GetRun(ctx, id)
}

func (r Repository) ListTemplateRuns(ctx context.Context, templateID int64, limit int) ([]app.RunSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT run.id,run.template_id,run.template_version,version.name,run.status,run.created_at,run.updated_at
		FROM %s.product_creator_runs run
		JOIN %s.business_templates template ON template.id=run.template_id
		LEFT JOIN %s.business_template_versions version ON version.template_id=run.template_id AND version.version=run.template_version
		WHERE ($1=0 OR run.template_id=$1) ORDER BY run.created_at DESC,run.id DESC LIMIT $2
	`, r.schema, r.schema, r.schema), templateID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.RunSummary, 0)
	for rows.Next() {
		var row app.RunSummary
		if err := rows.Scan(&row.ID, &row.TemplateID, &row.TemplateVersion, &row.TemplateName, &row.Status, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) GetRun(ctx context.Context, id int64) (app.Run, error) {
	run, err := scanRun(r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,template_id,template_version,status,revision,workflow_snapshot,inputs,preview,created_at,updated_at,commit_result FROM %s.product_creator_runs WHERE id=$1`, r.schema), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Run{}, app.ErrNotFound
	}
	return run, err
}

func (r Repository) SaveRunInputs(ctx context.Context, id, revision int64, inputs map[string]map[string]any, actor string) (app.Run, error) {
	body, err := json.Marshal(inputs)
	if err != nil {
		return app.Run{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_creator_runs SET inputs=$3::jsonb,preview=NULL,revision=revision+1,updated_at=now() WHERE id=$1 AND revision=$2 AND status='draft'`, r.schema), id, revision, body)
	if err != nil {
		return app.Run{}, err
	}
	if result.RowsAffected() == 0 {
		return app.Run{}, r.resolveRunWriteConflict(ctx, tx, id)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_creator_run", &id, "save_draft", postgresinfra.StrPtr("inputs"), nil, postgresinfra.StrPtr("saved"), postgresinfra.AuditMeta{"run_id": id, "revision": revision + 1}); err != nil {
		return app.Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.Run{}, err
	}
	return r.GetRun(ctx, id)
}

func (r Repository) SaveRunPreview(ctx context.Context, id, revision int64, preview app.RunPreview, actor string) (app.Run, error) {
	body, err := json.Marshal(preview)
	if err != nil {
		return app.Run{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_creator_runs SET preview=$3::jsonb,updated_at=now() WHERE id=$1 AND revision=$2 AND status='draft'`, r.schema), id, revision, body)
	if err != nil {
		return app.Run{}, err
	}
	if result.RowsAffected() == 0 {
		return app.Run{}, r.resolveRunWriteConflict(ctx, tx, id)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_creator_run", &id, "preview", postgresinfra.StrPtr("preview"), nil, postgresinfra.StrPtr("generated"), postgresinfra.AuditMeta{"run_id": id, "valid": preview.Valid, "step_count": len(preview.Steps), "issue_count": len(preview.Issues)}); err != nil {
		return app.Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.Run{}, err
	}
	return r.GetRun(ctx, id)
}

func (r Repository) CommitConfiguration(ctx context.Context, id int64, revision int64, idempotencyKey, requestHash, actor string, execute func(context.Context, app.Run) (map[string]any, error)) (app.Run, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var commitKey, commitHash string
	run, err := scanRun(tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,template_id,template_version,status,revision,workflow_snapshot,inputs,preview,created_at,updated_at,commit_result FROM %s.product_creator_runs WHERE id=$1 FOR UPDATE`, r.schema), id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.Run{}, app.ErrNotFound
		}
		return app.Run{}, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT commit_key,commit_hash FROM %s.product_creator_runs WHERE id=$1`, r.schema), id).Scan(&commitKey, &commitHash); err != nil {
		return app.Run{}, err
	}
	if commitKey != "" {
		if commitKey != idempotencyKey || commitHash != requestHash {
			return app.Run{}, app.ErrConflict
		}
		if run.Status != "config_committed" && run.Status != "in_progress" && run.Status != "completed" {
			return app.Run{}, app.ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return app.Run{}, err
		}
		return run, nil
	}
	if run.Revision != revision || run.Status != "draft" {
		return app.Run{}, app.ErrConflict
	}
	if run.Preview == nil || !run.Preview.Valid {
		return app.Run{}, app.InvalidWorkflowError{Issues: []app.ValidationIssue{{Code: "preview_required", Message: "请先完成有效的业务预览"}}}
	}
	result, err := execute(postgresinfra.WithTransaction(ctx, tx), run)
	if err != nil {
		return app.Run{}, err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return app.Run{}, err
	}
	steps := map[string]map[string]any{}
	if raw, ok := result["steps"].(map[string]map[string]any); ok {
		steps = raw
	} else if raw, ok := result["steps"].(map[string]any); ok {
		for nodeID, value := range raw {
			if step, ok := value.(map[string]any); ok {
				steps[nodeID] = step
			}
		}
	}
	for _, previewStep := range run.Preview.Steps {
		status := "succeeded"
		if previewStep.Status == "skipped" {
			status = "skipped"
		}
		if step, ok := steps[previewStep.NodeID]; ok {
			if requestedStatus, ok := step["status"].(string); ok && requestedStatus != "" {
				status = requestedStatus
			}
		}
		inputJSON, marshalErr := json.Marshal(run.Inputs[previewStep.NodeID])
		if marshalErr != nil {
			return app.Run{}, marshalErr
		}
		outputBytes, marshalErr := json.Marshal(steps[previewStep.NodeID])
		if marshalErr != nil {
			return app.Run{}, marshalErr
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_creator_run_steps(run_id,node_id,row_id,status,input_json,output_json,idempotency_key,request_hash,updated_at)
			VALUES($1,$2,'',$3,$4::jsonb,$5::jsonb,$6,$7,now())
			ON CONFLICT(run_id,node_id,row_id) DO UPDATE SET status=excluded.status,input_json=excluded.input_json,output_json=excluded.output_json,idempotency_key=excluded.idempotency_key,request_hash=excluded.request_hash,error_json=NULL,updated_at=now()`, r.schema), id, previewStep.NodeID, status, inputJSON, outputBytes, idempotencyKey+":"+previewStep.NodeID, requestHash); err != nil {
			return app.Run{}, err
		}
	}
	businessStatus := "config_committed"
	if requestedStatus, ok := result["run_status"].(string); ok && (requestedStatus == "in_progress" || requestedStatus == "config_committed") {
		businessStatus = requestedStatus
	}
	resultTag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_creator_runs SET status=$3,commit_key=$4,commit_hash=$5,commit_result=$6::jsonb,revision=revision+1,updated_at=now() WHERE id=$1 AND revision=$2 AND status='draft'`, r.schema), id, revision, businessStatus, idempotencyKey, requestHash, resultJSON)
	if err != nil {
		return app.Run{}, err
	}
	if resultTag.RowsAffected() != 1 {
		return app.Run{}, app.ErrConflict
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_creator_run", &id, "commit_configuration", postgresinfra.StrPtr("business_objects"), nil, postgresinfra.StrPtr("created"), postgresinfra.AuditMeta{"run_id": id, "template_id": run.TemplateID, "template_version": run.Version, "request_hash": requestHash, "step_count": len(run.Preview.Steps)}); err != nil {
		return app.Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.Run{}, err
	}
	return r.GetRun(ctx, id)
}

func (r Repository) ExecuteRunStep(ctx context.Context, id, revision int64, nodeID, action, idempotencyKey, requestHash, actor string, stepInputs map[string]any, execute func(context.Context, app.Run, app.Node, string, map[string]any) (map[string]any, error)) (app.Run, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	run, err := scanRun(tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,template_id,template_version,status,revision,workflow_snapshot,inputs,preview,created_at,updated_at,commit_result FROM %s.product_creator_runs WHERE id=$1 FOR UPDATE`, r.schema), id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.Run{}, app.ErrNotFound
		}
		return app.Run{}, err
	}
	var existingNode, existingAction, existingHash string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT node_id,row_id,request_hash FROM %s.product_creator_run_steps WHERE run_id=$1 AND idempotency_key=$2 FOR UPDATE`, r.schema), id, idempotencyKey).Scan(&existingNode, &existingAction, &existingHash)
	if err == nil {
		if existingNode != nodeID || existingAction != action || existingHash != requestHash {
			return app.Run{}, app.ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return app.Run{}, err
		}
		return run, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return app.Run{}, err
	}
	if run.Revision != revision || run.Status != "in_progress" {
		return app.Run{}, app.ErrConflict
	}
	var node app.Node
	found := false
	for _, candidate := range run.Workflow.Nodes {
		if candidate.ID == nodeID {
			node, found = candidate, true
			break
		}
	}
	if !found {
		return app.Run{}, app.ErrNotFound
	}
	result, err := execute(postgresinfra.WithBusinessProvenance(postgresinfra.WithTransaction(ctx, tx), id, nodeID), run, node, action, stepInputs)
	if err != nil {
		return app.Run{}, err
	}
	if result == nil {
		result = map[string]any{}
	}
	steps := resultMap(run.BusinessResults["steps"])
	mergedStep := resultMap(steps[nodeID])
	for key, value := range result {
		mergedStep[key] = value
	}
	steps[nodeID] = mergedStep
	businessResults := run.BusinessResults
	if businessResults == nil {
		businessResults = map[string]any{}
	}
	businessResults["steps"] = steps
	if allWorkflowFollowupsComplete(run.Workflow, steps) {
		run.Status = "completed"
		businessResults["run_status"] = "completed"
	}
	resultJSON, err := json.Marshal(businessResults)
	if err != nil {
		return app.Run{}, err
	}
	inputJSON, err := json.Marshal(stepInputs)
	if err != nil {
		return app.Run{}, err
	}
	outputJSON, err := json.Marshal(result)
	if err != nil {
		return app.Run{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_creator_run_steps(run_id,node_id,row_id,status,input_json,output_json,idempotency_key,request_hash,updated_at)
		VALUES($1,$2,$3,'succeeded',$4::jsonb,$5::jsonb,$6,$7,now())
		ON CONFLICT(run_id,node_id,row_id) DO UPDATE SET status='succeeded',input_json=excluded.input_json,output_json=excluded.output_json,idempotency_key=excluded.idempotency_key,request_hash=excluded.request_hash,error_json=NULL,updated_at=now()`, r.schema), id, nodeID, action, inputJSON, outputJSON, idempotencyKey, requestHash); err != nil {
		return app.Run{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_creator_runs SET status=$2,commit_result=$3::jsonb,revision=revision+1,updated_at=now() WHERE id=$1`, r.schema), id, run.Status, resultJSON); err != nil {
		return app.Run{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_creator_run", &id, "execute_step", postgresinfra.StrPtr("node_id"), nil, postgresinfra.StrPtr(nodeID), postgresinfra.AuditMeta{"run_id": id, "node_id": nodeID, "action": action, "request_hash": requestHash}); err != nil {
		return app.Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.Run{}, err
	}
	return r.GetRun(ctx, id)
}

func resultMap(value any) map[string]any {
	if values, ok := value.(map[string]any); ok {
		return values
	}
	return map[string]any{}
}

func allWorkflowFollowupsComplete(workflow app.Workflow, steps map[string]any) bool {
	found := false
	for _, node := range workflow.Nodes {
		if node.Kind != app.ModulePurchase && node.Kind != app.ModulePricing {
			continue
		}
		found = true
		step := resultMap(steps[node.ID])
		if step["status"] == "skipped" {
			continue
		}
		switch node.Kind {
		case app.ModulePurchase:
			if step["purchase_receipt"] == nil {
				return false
			}
		case app.ModulePricing:
			if step["published_price"] == nil {
				return false
			}
		}
	}
	return found
}

func (r Repository) getVersion(ctx context.Context, id int64) (app.TemplateVersion, error) {
	return scanVersion(r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,template_id,version,name,description,workflow,published_at,published_by FROM %s.business_template_versions WHERE id=$1`, r.schema), id))
}

func (r Repository) resolveTemplateWriteConflict(ctx context.Context, tx pgx.Tx, id int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_templates WHERE id=$1)`, r.schema), id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return app.ErrNotFound
	}
	return app.ErrConflict
}

func (r Repository) resolveRunWriteConflict(ctx context.Context, tx pgx.Tx, id int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_creator_runs WHERE id=$1)`, r.schema), id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return app.ErrNotFound
	}
	return app.ErrConflict
}

type rowScanner interface{ Scan(...any) error }

func scanTemplate(row rowScanner) (app.Template, error) {
	var out app.Template
	var graph []byte
	err := row.Scan(&out.ID, &out.Name, &out.Description, &out.Status, &out.Revision, &out.PublishedVersion, &graph, &out.UpdatedAt)
	if err != nil {
		return app.Template{}, err
	}
	if err := json.Unmarshal(graph, &out.Draft); err != nil {
		return app.Template{}, err
	}
	return out, nil
}

func scanVersion(row rowScanner) (app.TemplateVersion, error) {
	var out app.TemplateVersion
	var workflow []byte
	err := row.Scan(&out.ID, &out.TemplateID, &out.Version, &out.Name, &out.Description, &workflow, &out.PublishedAt, &out.PublishedBy)
	if err != nil {
		return app.TemplateVersion{}, err
	}
	if err := json.Unmarshal(workflow, &out.Workflow); err != nil {
		return app.TemplateVersion{}, err
	}
	return out, nil
}

func scanRun(row rowScanner) (app.Run, error) {
	var out app.Run
	var workflow, inputs, preview, businessResults []byte
	err := row.Scan(&out.ID, &out.TemplateID, &out.Version, &out.Status, &out.Revision, &workflow, &inputs, &preview, &out.CreatedAt, &out.UpdatedAt, &businessResults)
	if err != nil {
		return app.Run{}, err
	}
	if err := json.Unmarshal(workflow, &out.Workflow); err != nil {
		return app.Run{}, err
	}
	if err := json.Unmarshal(inputs, &out.Inputs); err != nil {
		return app.Run{}, err
	}
	if out.Inputs == nil {
		out.Inputs = map[string]map[string]any{}
	}
	if len(preview) > 0 && string(preview) != "null" {
		var value app.RunPreview
		if err := json.Unmarshal(preview, &value); err != nil {
			return app.Run{}, err
		}
		out.Preview = &value
	}
	if len(businessResults) > 0 && string(businessResults) != "null" {
		if err := json.Unmarshal(businessResults, &out.BusinessResults); err != nil {
			return app.Run{}, err
		}
	}
	return out, nil
}

var _ app.Repository = Repository{}

func normalizeActor(actor string) string { return strings.TrimSpace(actor) }
