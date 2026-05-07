package authz

import (
	"context"
	"testing"
)

type fakeRepo struct {
	actor       Actor
	assignments map[int64][]string
}

func (f fakeRepo) ActorByEmployeeID(ctx context.Context, employeeID int64) (Actor, error) {
	return f.actor, nil
}

func (f fakeRepo) ListRoles(ctx context.Context) ([]Role, error) {
	return []Role{{Code: "admin", Name: "管理员"}, {Code: "sales", Name: "销售"}}, nil
}

func (f fakeRepo) AssignEmployeeRoles(ctx context.Context, cmd AssignmentCommand) error {
	return nil
}

func (f fakeRepo) ListEmployeeRoles(ctx context.Context) (map[int64][]string, error) {
	return f.assignments, nil
}

func TestAdminCanAccessEveryPermissionAndView(t *testing.T) {
	svc := NewService(fakeRepo{actor: Actor{
		EmployeeID: 1,
		Name:       "Van",
		Roles:      []Role{{Code: "admin", Name: "管理员"}},
	}})

	actor, err := svc.ActorByEmployeeID(context.Background(), 1)
	if err != nil {
		t.Fatalf("ActorByEmployeeID: %v", err)
	}

	if !actor.Can("settings.write") {
		t.Fatal("admin should be allowed to write settings")
	}
	if !actor.CanView("senderSettings") {
		t.Fatal("admin should be allowed to open every view")
	}
}

func TestRolePermissionsAreMergedAndDeduplicated(t *testing.T) {
	svc := NewService(fakeRepo{actor: Actor{
		EmployeeID: 2,
		Name:       "Sales",
		Roles: []Role{
			{Code: "sales", Permissions: []string{"orders.read", "orders.write"}},
			{Code: "customer", Permissions: []string{"orders.read", "customers.read"}},
		},
		ViewPermissions: map[string]string{
			"orders":    "orders.read",
			"customers": "customers.read",
			"machines":  "settings.write",
		},
	}})

	actor, err := svc.ActorByEmployeeID(context.Background(), 2)
	if err != nil {
		t.Fatalf("ActorByEmployeeID: %v", err)
	}

	if !actor.Can("orders.write") || !actor.Can("customers.read") {
		t.Fatalf("merged permissions missing: %+v", actor.Permissions)
	}
	if actor.Can("settings.write") {
		t.Fatal("sales actor unexpectedly has settings.write")
	}
	if got := actor.AllowedViews(); len(got) != 2 || got[0] != "customers" || got[1] != "orders" {
		t.Fatalf("AllowedViews = %#v, want sorted customers/orders", got)
	}
}

func TestAssignmentCommandNormalizesRoleCodes(t *testing.T) {
	cmd := AssignmentCommand{EmployeeID: 9, RoleCodes: []string{" sales ", "", "admin", "sales"}}
	got, err := cmd.Normalized()
	if err != nil {
		t.Fatalf("Normalized: %v", err)
	}
	if got.EmployeeID != 9 {
		t.Fatalf("EmployeeID = %d, want 9", got.EmployeeID)
	}
	if len(got.RoleCodes) != 2 || got.RoleCodes[0] != "admin" || got.RoleCodes[1] != "sales" {
		t.Fatalf("RoleCodes = %#v, want admin/sales", got.RoleCodes)
	}
}

func TestListEmployeeRolesReturnsRepositoryAssignments(t *testing.T) {
	want := map[int64][]string{9: {"sales"}}
	svc := NewService(fakeRepo{assignments: want})
	got, err := svc.ListEmployeeRoles(context.Background())
	if err != nil {
		t.Fatalf("ListEmployeeRoles: %v", err)
	}
	if got[9][0] != "sales" {
		t.Fatalf("assignments=%+v", got)
	}
}
