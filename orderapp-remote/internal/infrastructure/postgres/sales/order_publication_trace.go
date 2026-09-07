package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Enrich historical reads without updating orders; new writes freeze metadata
// from the selected publication rather than trusting client-provided labels.
func orderPublicationTraceSnapshot(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, schema string, id int64, raw string, preserve bool) (string, error) {
	if id <= 0 {
		return raw, nil
	}
	source := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &source)
	if source == nil {
		source = map[string]any{}
	}
	if preserve && source["price_list_published_at"] != nil {
		return raw, nil
	}
	var name, ownerType, ownerKey, ownerName, version, publishedAt string
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(b.product_type_name,''),b.owner_type,b.owner_key,
 CASE WHEN b.owner_type='official' THEN '工厂公共' ELSE COALESCE(c.name,'客户 #'||b.owner_key) END,
 COALESCE(b.version_no,''),to_char(b.published_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS')
 FROM %[1]s.bean_list_publications b LEFT JOIN %[1]s.customers c ON c.id::text=b.owner_key AND b.owner_type='customer' WHERE b.id=$1`, schema), id).Scan(&name, &ownerType, &ownerKey, &ownerName, &version, &publishedAt)
	if err == pgx.ErrNoRows {
		return raw, nil
	}
	if err != nil {
		return "", err
	}
	source["price_list_name"] = name
	source["price_list_owner_type"] = ownerType
	source["price_list_owner_key"] = ownerKey
	source["price_list_owner_name"] = ownerName
	source["price_list_published_at"] = publishedAt
	source["version"] = version
	source["publication_id"] = id
	result, err := json.Marshal(source)
	return string(result), err
}
