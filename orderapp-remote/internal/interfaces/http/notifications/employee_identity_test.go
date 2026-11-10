package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestWechatEmployeeNotificationIdentity(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("notification_employee_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	for _, sql := range []string{`CREATE TABLE %s.company_employees(id bigint PRIMARY KEY,phone text,account_type text,active boolean,updated_at timestamptz)`, `CREATE TABLE %s.employee_login_passwords(employee_id bigint,login_disabled boolean,updated_at timestamptz)`, `CREATE TABLE %s.mini_users(id bigint PRIMARY KEY,openid text,active boolean)`} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(sql, schema)); err != nil {
			t.Fatal(err)
		}
	}
	phone := "13960009998"
	sum := sha256.Sum256([]byte(phone))
	openid := fmt.Sprintf("wechat-internal-employee:1:%s", hex.EncodeToString(sum[:]))
	boundAt := time.Now()
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees VALUES(1,$1,'internal_employee',true,now()-interval '1 minute')`, schema), phone); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users VALUES(1,$1,true)`, schema), openid); err != nil {
		t.Fatal(err)
	}
	check := func(want bool) {
		t.Helper()
		got, err := employeeNotificationAvailable(ctx, pool, schema, 1, 1, boundAt)
		if err != nil || got != want {
			t.Fatalf("available=%v want=%v err=%v", got, want, err)
		}
	}
	check(true)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees VALUES(2,$1,'channel_customer',true,now()-interval '1 minute')`, schema), phone); err != nil {
		t.Fatal(err)
	}
	check(false)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.company_employees WHERE id=2`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords VALUES(1,true,now())`, schema)); err != nil {
		t.Fatal(err)
	}
	check(false)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.employee_login_passwords WHERE employee_id=1`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.company_employees SET phone='13960009997',updated_at=now() WHERE id=1`, schema)); err != nil {
		t.Fatal(err)
	}
	check(false)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.company_employees SET phone=$1,updated_at=now() WHERE id=1`, schema), phone); err != nil {
		t.Fatal(err)
	}
	check(false)
}
