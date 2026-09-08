package support

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestERPChannelCustomerAuthWithBoundedPool(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	employeeID, _, token := seedValidERPBearer(t, context.Background(), pool, schema, "bounded pool customer", "13942000001")
	for _, size := range []int32{1, 4} {
		t.Run(fmt.Sprintf("connections_%d", size), func(t *testing.T) {
			config := pool.Config()
			config.MaxConns = size
			limited, err := pgxpool.NewWithConfig(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer limited.Close()
			service := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(limited, schema))
			t.Run("portal_context", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := service.RequireERPWorkbenchLogin(ctx, employeeID); err != nil {
					t.Fatalf("valid customer must resolve without holding a second pool connection: %v", err)
				}
			})
			t.Run("concurrent_bearer_requests", func(t *testing.T) {
				server := newERPBearerTestServer(limited, schema, service)
				var wg sync.WaitGroup
				results := make(chan int, 8)
				start := make(chan struct{})
				for i := 0; i < cap(results); i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						<-start
						ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
						defer cancel()
						req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil).WithContext(ctx)
						req.Header.Set("Authorization", "Bearer "+token)
						rec := httptest.NewRecorder()
						server.ServeHTTP(rec, req)
						results <- rec.Code
					}()
				}
				close(start)
				wg.Wait()
				close(results)
				for status := range results {
					if status != http.StatusOK {
						t.Errorf("concurrent valid bearer status=%d, want 200", status)
					}
				}
			})
		})
	}
}

type erpPoolEligibilityFunc func(context.Context, int64) error

func (f erpPoolEligibilityFunc) RequireERPWorkbenchLogin(ctx context.Context, employeeID int64) error {
	return f(ctx, employeeID)
}

func TestERPChannelCustomerAuthRechecksSecurityAfterEligibility(t *testing.T) {
	pool, schema := newERPLoginGateTestDB(t)
	service := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	for i, change := range []string{"logout", "password", "account_type"} {
		t.Run(change, func(t *testing.T) {
			employeeID, _, token := seedValidERPBearer(t, context.Background(), pool, schema, change, fmt.Sprintf("13942001%03d", i))
			eligibility := erpPoolEligibilityFunc(func(ctx context.Context, id int64) error {
				if err := service.RequireERPWorkbenchLogin(ctx, id); err != nil {
					return err
				}
				var err error
				switch change {
				case "logout":
					_, err = pool.Exec(ctx, "DELETE FROM "+schema+".login_sessions WHERE token=$1", token)
				case "password":
					_, err = pool.Exec(ctx, "UPDATE "+schema+".employee_login_passwords SET updated_at=now() WHERE employee_id=$1", employeeID)
				case "account_type":
					_, err = pool.Exec(ctx, "UPDATE "+schema+".company_employees SET account_type='internal_employee' WHERE id=$1", employeeID)
				}
				return err
			})
			server := newERPBearerTestServer(pool, schema, eligibility)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("security change during eligibility returned %d, want 401", rec.Code)
			}
			assertERPLoginTokenCount(t, context.Background(), pool, schema, token, 0)
		})
	}
}
