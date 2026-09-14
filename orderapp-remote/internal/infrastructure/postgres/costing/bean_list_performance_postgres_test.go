package costing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	appcosting "orderapp/internal/application/costing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBeanListPerformanceAgainstConfiguredDatabase(t *testing.T) {
	if os.Getenv("ORDERAPP_COSTING_PERFORMANCE_TEST") != "1" {
		t.Skip("set ORDERAPP_COSTING_PERFORMANCE_TEST=1 to run price-list performance acceptance")
	}
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	schema := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_SCHEMA"))
	customerID, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("ORDERAPP_TEST_CUSTOMER_ID")), 10, 64)
	if dsn == "" || schema == "" || err != nil || customerID <= 0 {
		t.Fatal("ORDERAPP_TEST_DATABASE_URL, ORDERAPP_TEST_SCHEMA and a positive ORDERAPP_TEST_CUSTOMER_ID are required")
	}

	type scope struct {
		name       string
		customerID int64
	}
	referenceHashByScope := map[string]string{}
	for _, planMode := range []string{"force_custom_plan", "force_generic_plan"} {
		config, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET plan_cache_mode = "+planMode)
			return err
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), config)
		if err != nil {
			t.Fatal(err)
		}
		service := appcosting.NewService(NewRepository(pool, schema))

		for _, current := range []scope{{name: "customer", customerID: customerID}, {name: "public"}} {
			t.Run(planMode+"/"+current.name, func(t *testing.T) {
				durations := make([]time.Duration, 0, 20)
				responseHash := ""
				for attempt := 0; attempt < 20; attempt++ {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					started := time.Now()
					response, err := service.BeanList(ctx, appcosting.BeanListQuery{CustomerID: current.customerID})
					durations = append(durations, time.Since(started))
					cancel()
					if err != nil {
						t.Fatalf("attempt %d: %v", attempt+1, err)
					}
					if len(response.Items) == 0 {
						t.Fatalf("attempt %d returned no products", attempt+1)
					}
					if current.customerID > 0 {
						assertExpectedPerformanceProduct(t, response, attempt)
					}
					currentHash := calculateResponseHash(t, response)
					if responseHash == "" {
						responseHash = currentHash
					} else if currentHash != responseHash {
						t.Fatalf("attempt %d response hash = %s, first response = %s", attempt+1, currentHash, responseHash)
					}
				}
				if reference := referenceHashByScope[current.name]; reference == "" {
					referenceHashByScope[current.name] = responseHash
				} else if responseHash != reference {
					t.Fatalf("%s response hash = %s, custom/generic reference = %s", planMode, responseHash, reference)
				}
				firstDuration := durations[0]
				sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
				p95 := durations[18]
				t.Logf("runs=20 first=%s min=%s median=%s p95=%s max=%s response_sha256=%s", firstDuration, durations[0], durations[10], p95, durations[19], responseHash)
				if p95 > time.Second {
					t.Fatalf("P95 = %s, want <= 1s", p95)
				}
			})
		}
		pool.Close()
	}
}

func TestMaterializedBOMUnitCostScalesInIsolatedDatabase(t *testing.T) {
	if os.Getenv("ORDERAPP_COSTING_PERFORMANCE_TEST") != "1" {
		t.Skip("set ORDERAPP_COSTING_PERFORMANCE_TEST=1 to run price-list performance acceptance")
	}
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Fatal("ORDERAPP_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(context.Background(), `CREATE TEMP TABLE bean_list_bom_scale (
		product_id bigint NOT NULL,
		component_cost numeric NOT NULL
	) ON COMMIT PRESERVE ROWS`); err != nil {
		t.Fatal(err)
	}

	var nearDuration time.Duration
	for _, current := range []struct {
		name string
		rows int
	}{{name: "production_scale", rows: 31400}, {name: "tenfold_scale", rows: 314000}} {
		if _, err := conn.Exec(context.Background(), "TRUNCATE bean_list_bom_scale"); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(context.Background(), `INSERT INTO bean_list_bom_scale (product_id, component_cost)
			SELECT ((n - 1) % 314) + 1, ((n % 97) + 1)::numeric / 100
			FROM generate_series(1, $1) AS n`, current.rows); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(context.Background(), "ANALYZE bean_list_bom_scale"); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		var planJSON []byte
		err := conn.QueryRow(context.Background(), `EXPLAIN (ANALYZE, FORMAT JSON)
			WITH bom_unit_cost AS MATERIALIZED (
				SELECT product_id, SUM(component_cost) AS unit_cost
				FROM bean_list_bom_scale
				GROUP BY product_id
			)
			SELECT COUNT(*), SUM(a.unit_cost + b.unit_cost)
			FROM bom_unit_cost a
			JOIN bom_unit_cost b USING (product_id)`).Scan(&planJSON)
		duration := time.Since(started)
		if err != nil {
			t.Fatal(err)
		}
		loops, ok := materializedCTEActualLoops(t, planJSON, "CTE bom_unit_cost")
		if !ok {
			t.Fatalf("%s plan missing materialized BOM CTE", current.name)
		}
		if loops != 1 {
			t.Fatalf("%s BOM aggregation loops = %.0f, want 1", current.name, loops)
		}
		if current.name == "production_scale" {
			nearDuration = duration
			t.Logf("%s rows=%d duration=%s aggregation_loops=%.0f", current.name, current.rows, duration, loops)
		} else {
			t.Logf("%s rows=%d duration=%s growth=%.2fx aggregation_loops=%.0f", current.name, current.rows, duration, float64(duration)/float64(nearDuration), loops)
		}
	}
}

func calculateResponseHash(t *testing.T, response *appcosting.CalculateResponse) string {
	t.Helper()
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func materializedCTEActualLoops(t *testing.T, raw []byte, subplanName string) (float64, bool) {
	t.Helper()
	var plans []map[string]any
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("EXPLAIN returned %d plans, want 1", len(plans))
	}
	root, ok := plans[0]["Plan"].(map[string]any)
	if !ok {
		t.Fatalf("EXPLAIN plan has unexpected shape: %s", fmt.Sprintf("%.200s", raw))
	}
	var find func(map[string]any) (float64, bool)
	find = func(node map[string]any) (float64, bool) {
		if node["Subplan Name"] == subplanName {
			loops, ok := node["Actual Loops"].(float64)
			return loops, ok
		}
		children, _ := node["Plans"].([]any)
		for _, child := range children {
			childNode, ok := child.(map[string]any)
			if !ok {
				continue
			}
			if loops, found := find(childNode); found {
				return loops, true
			}
		}
		return 0, false
	}
	return find(root)
}

func assertExpectedPerformanceProduct(t *testing.T, response *appcosting.CalculateResponse, attempt int) {
	t.Helper()
	expectedName := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_EXPECT_PRODUCT_NAME"))
	expectedCount, err := strconv.Atoi(strings.TrimSpace(os.Getenv("ORDERAPP_TEST_EXPECT_PRODUCT_COUNT")))
	if expectedName == "" || err != nil || expectedCount <= 0 {
		return
	}
	count := 0
	for _, item := range response.Items {
		if strings.TrimSpace(item.Name) == expectedName {
			count++
		}
	}
	if count != expectedCount {
		t.Fatalf("attempt %d product %q rows = %d, want %d", attempt+1, expectedName, count, expectedCount)
	}
}
