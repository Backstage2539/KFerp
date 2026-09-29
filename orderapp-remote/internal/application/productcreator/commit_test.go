package productcreator

import (
	"context"
	"strings"
	"testing"
)

type commitTestExecutor struct{ calls int }

func (e *commitTestExecutor) ExecuteConfiguration(context.Context, Run, string) (map[string]any, error) {
	e.calls++
	return map[string]any{"created": true}, nil
}

type commitTestRepository struct {
	Repository
	run      Run
	called   bool
	key      string
	hash     string
	callback func(context.Context, Run) (map[string]any, error)
}

type v3DraftFallbackRepository struct {
	Repository
	run         Run
	legacySaved bool
}

type v3DraftRepository struct {
	Repository
	run            Run
	savedInputs    map[string]map[string]any
	savedVariables map[string]string
}

func (r *v3DraftRepository) GetRun(context.Context, int64) (Run, error) { return r.run, nil }
func (r *v3DraftRepository) SaveRunDraft(_ context.Context, _ int64, revision int64, inputs map[string]map[string]any, variables map[string]string, _ string) (Run, error) {
	r.savedInputs, r.savedVariables = inputs, variables
	r.run.Inputs, r.run.VariableValues, r.run.Revision = inputs, variables, revision+1
	return r.run, nil
}

func (r *v3DraftFallbackRepository) GetRun(context.Context, int64) (Run, error) { return r.run, nil }
func (r *v3DraftFallbackRepository) SaveRunInputs(_ context.Context, _ int64, revision int64, inputs map[string]map[string]any, _ string) (Run, error) {
	r.legacySaved = true
	r.run.Inputs = inputs
	r.run.Revision = revision + 1
	return r.run, nil
}

type runStepTestRepository struct {
	Repository
	run      Run
	called   bool
	key      string
	hash     string
	callback func(context.Context, Run, Node, string, map[string]any) (map[string]any, error)
}

func (r *runStepTestRepository) GetRun(context.Context, int64) (Run, error) { return r.run, nil }
func (r *runStepTestRepository) ExecuteRunStep(_ context.Context, _ int64, _ int64, _ string, _ string, key, hash, _ string, _ map[string]any, execute func(context.Context, Run, Node, string, map[string]any) (map[string]any, error)) (Run, error) {
	r.called, r.key, r.hash, r.callback = true, key, hash, execute
	var node Node
	for _, candidate := range r.run.Workflow.Nodes {
		if candidate.ID == "pricing" {
			node = candidate
		}
	}
	_, err := execute(context.Background(), r.run, node, strings.TrimPrefix(key, "idem-"), nil)
	return r.run, err
}

type runStepTestExecutor struct{ action string }

func (e *runStepTestExecutor) ExecuteRunStep(_ context.Context, _ Run, _ Node, action string, _ map[string]any, _ string) (map[string]any, error) {
	e.action = action
	return map[string]any{"status": "succeeded"}, nil
}

func (r *commitTestRepository) GetRun(context.Context, int64) (Run, error) { return r.run, nil }
func (r *commitTestRepository) CommitConfiguration(_ context.Context, _ int64, _ int64, key, hash, _ string, execute func(context.Context, Run) (map[string]any, error)) (Run, error) {
	r.called, r.key, r.hash, r.callback = true, key, hash, execute
	result, err := execute(context.Background(), r.run)
	if err != nil {
		return Run{}, err
	}
	r.run.Status = "config_committed"
	r.run.Preview = &RunPreview{Valid: true}
	_ = result
	return r.run, nil
}

func TestCommitConfigurationRequiresValidFreshPreview(t *testing.T) {
	repo := &commitTestRepository{run: Run{ID: 8, Revision: 3, Status: "draft", Workflow: Workflow{Nodes: []Node{{ID: "p", Kind: ModuleProduct}}}, Inputs: map[string]map[string]any{"p": {"name": "测试商品", "action": "create", "owner": "factory"}}}}
	svc := NewService(repo)
	executor := &commitTestExecutor{}
	svc.UseConfigurationExecutor(executor)
	if _, err := svc.CommitConfiguration(context.Background(), 8, 3, "request-1", "van"); err == nil {
		t.Fatal("commit without preview should fail")
	}
	if repo.called || executor.calls != 0 {
		t.Fatal("configuration executor must not run before preview validation")
	}
}

func TestCommitConfigurationUsesIdempotencyKeyAndTransactionalExecutor(t *testing.T) {
	run := Run{
		ID: 8, Revision: 3, Status: "draft",
		Workflow: Workflow{Nodes: []Node{{ID: "p", Kind: ModuleProduct}}},
		Inputs:   map[string]map[string]any{"p": {"name": "测试商品", "action": "create", "owner": "factory"}},
		Preview:  &RunPreview{Valid: true},
	}
	repo := &commitTestRepository{run: run}
	svc := NewService(repo)
	executor := &commitTestExecutor{}
	svc.UseConfigurationExecutor(executor)
	got, err := svc.CommitConfiguration(context.Background(), 8, 3, "request-1", "van")
	if err != nil {
		t.Fatal(err)
	}
	if !repo.called || repo.key != "request-1" || len(repo.hash) != 64 || executor.calls != 1 || got.Status != "config_committed" {
		t.Fatalf("commit did not pass through the idempotent transaction: called=%v key=%q hash=%q calls=%d run=%+v", repo.called, repo.key, repo.hash, executor.calls, got)
	}
}

func TestCommitConfigurationRequestHashIncludesV3VariableValues(t *testing.T) {
	workflow := Workflow{Version: 3, Variables: []WorkflowVariable{{ID: "batch-name", Name: "批次名称"}}, Nodes: []Node{{ID: "materials", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}}}}
	inputs := map[string]map[string]any{"materials": {"rows": []any{map[string]any{"row_id": "material-1", "action": "create", "name": "咖啡生豆", "unit": "kg", "owner_type": "factory"}}}}
	hashes := make([]string, 0, 2)
	for _, variableValue := range []string{"第一批", "第二批"} {
		run := Run{ID: 82, Revision: 4, Status: "draft", Workflow: workflow, Inputs: inputs, VariableValues: map[string]string{"batch-name": variableValue}, Preview: &RunPreview{Valid: true}}
		repo := &commitTestRepository{run: run}
		svc := NewService(repo)
		svc.UseConfigurationExecutor(&commitTestExecutor{})
		if _, err := svc.CommitConfiguration(context.Background(), run.ID, run.Revision, "same-key", "tester"); err != nil {
			t.Fatalf("commit with variable %q failed: %v", variableValue, err)
		}
		hashes = append(hashes, repo.hash)
	}
	if hashes[0] == hashes[1] {
		t.Fatal("different V3 variable values must produce different idempotency request hashes")
	}
}

func TestPricingStepSupportsPreviewDraftAndPublishActions(t *testing.T) {
	run := Run{ID: 19, Revision: 8, Status: "in_progress", Version: 3, Workflow: Workflow{Nodes: []Node{{ID: "pricing", Kind: ModulePricing}}}, Inputs: map[string]map[string]any{"pricing": {"price_list_id": 12, "prices": []any{map[string]any{"row_id": "spec-1", "price": 19.9}}}}}
	for _, action := range []string{"preview_pricing", "save_price_draft", "publish_price"} {
		t.Run(action, func(t *testing.T) {
			repo := &runStepTestRepository{run: run}
			svc := NewService(repo)
			executor := &runStepTestExecutor{}
			svc.UseRunStepExecutor(executor)
			_, err := svc.ExecuteRunStep(context.Background(), run.ID, run.Revision, "pricing", action, "idem-"+action, "tester", nil)
			if err != nil {
				t.Fatalf("ExecuteRunStep(%s) error: %v", action, err)
			}
			if !repo.called || executor.action != action || repo.key != "idem-"+action || len(repo.hash) != 64 {
				t.Fatalf("pricing action did not pass through the idempotent executor: repo=%+v executor=%+v", repo, executor)
			}
		})
	}
}

func TestV3DraftSaveDoesNotFallBackToInputsOnlyPersistence(t *testing.T) {
	repo := &v3DraftFallbackRepository{run: Run{
		ID: 41, Revision: 2, Status: "draft",
		Workflow: Workflow{Version: 3, Variables: []WorkflowVariable{{ID: "product-name", Name: "商品名称", DefaultValue: "默认商品"}}, Nodes: []Node{{ID: "input", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}}}},
		Inputs:   map[string]map[string]any{}, VariableValues: map[string]string{},
	}}
	_, err := NewService(repo).SaveRunDraft(context.Background(), 41, 2, map[string]map[string]any{}, map[string]string{"product-name": "本次商品"}, "tester")
	if err != ErrVariableDraftPersistenceUnavailable {
		t.Fatalf("V3 save error=%v, want explicit variable persistence error", err)
	}
	if repo.legacySaved {
		t.Fatal("V3 variables and node inputs must never be saved through the inputs-only fallback")
	}
}

func TestV3DraftSavePersistsVariableValuesWithResolvedNodeInputs(t *testing.T) {
	repo := &v3DraftRepository{run: Run{
		ID: 42, Revision: 3, Status: "draft",
		Workflow: Workflow{Version: 3, Variables: []WorkflowVariable{{ID: "product-name", Name: "商品名称", DefaultValue: "模板默认"}}, Nodes: []Node{{ID: "input", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}}}},
	}}
	got, err := NewService(repo).SaveRunDraft(context.Background(), 42, 3, map[string]map[string]any{}, map[string]string{"product-name": "本次商品"}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 4 || repo.savedVariables["product-name"] != "本次商品" {
		t.Fatalf("saved V3 draft revision/variables=%d/%v", got.Revision, repo.savedVariables)
	}
}
