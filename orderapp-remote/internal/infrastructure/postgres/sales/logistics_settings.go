package sales

import (
	"context"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) ListLogisticsCompanies(ctx context.Context, includeInactive bool) ([]salesapp.LogisticsCompany, error) {
	whereCompany := "WHERE active=true"
	whereProduct := "AND active=true"
	if includeInactive {
		whereCompany = ""
		whereProduct = ""
	}
	q := fmt.Sprintf(`SELECT id, name, sort, active
		FROM %s.logistics_companies
		%s
		ORDER BY sort, id`, r.schema, whereCompany)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	companies := make([]salesapp.LogisticsCompany, 0)
	index := map[int64]int{}
	for rows.Next() {
		var row salesapp.LogisticsCompany
		if err := rows.Scan(&row.ID, &row.Name, &row.Sort, &row.Active); err != nil {
			return nil, err
		}
		index[row.ID] = len(companies)
		companies = append(companies, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pq := fmt.Sprintf(`SELECT id, company_id, name, sort, active
		FROM %s.logistics_products
		WHERE company_id = ANY($1) %s
		ORDER BY company_id, sort, id`, r.schema, whereProduct)
	ids := make([]int64, 0, len(companies))
	for _, company := range companies {
		ids = append(ids, company.ID)
	}
	if len(ids) == 0 {
		return companies, nil
	}
	prows, err := r.pool.Query(ctx, pq, ids)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var product salesapp.LogisticsProduct
		if err := prows.Scan(&product.ID, &product.CompanyID, &product.Name, &product.Sort, &product.Active); err != nil {
			return nil, err
		}
		if idx, ok := index[product.CompanyID]; ok {
			companies[idx].Products = append(companies[idx].Products, product)
		}
	}
	return companies, prows.Err()
}

func (r Repository) SaveLogisticsCompany(ctx context.Context, cmd salesapp.SaveLogisticsCompanyCommand) (salesapp.LogisticsCompany, error) {
	var row salesapp.LogisticsCompany
	if cmd.ID > 0 {
		q := fmt.Sprintf(`UPDATE %s.logistics_companies
			SET name=$2, sort=$3, active=$4, updated_at=now()
			WHERE id=$1
			RETURNING id, name, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, cmd.ID, strings.TrimSpace(cmd.Name), cmd.Sort, cmd.Active).Scan(&row.ID, &row.Name, &row.Sort, &row.Active); err != nil {
			return salesapp.LogisticsCompany{}, err
		}
	} else {
		q := fmt.Sprintf(`INSERT INTO %s.logistics_companies(name, sort, active)
			VALUES($1,$2,$3)
			RETURNING id, name, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, strings.TrimSpace(cmd.Name), cmd.Sort, cmd.Active).Scan(&row.ID, &row.Name, &row.Sort, &row.Active); err != nil {
			return salesapp.LogisticsCompany{}, err
		}
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "logistics_company", &row.ID, "update", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(row.Name), postgresinfra.AuditMeta{"sort": row.Sort, "active": row.Active})
	return row, nil
}

func (r Repository) SaveLogisticsProduct(ctx context.Context, cmd salesapp.SaveLogisticsProductCommand) (salesapp.LogisticsProduct, error) {
	var row salesapp.LogisticsProduct
	if cmd.ID > 0 {
		q := fmt.Sprintf(`UPDATE %s.logistics_products
			SET company_id=$2, name=$3, sort=$4, active=$5, updated_at=now()
			WHERE id=$1
			RETURNING id, company_id, name, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, cmd.ID, cmd.CompanyID, strings.TrimSpace(cmd.Name), cmd.Sort, cmd.Active).Scan(&row.ID, &row.CompanyID, &row.Name, &row.Sort, &row.Active); err != nil {
			return salesapp.LogisticsProduct{}, err
		}
	} else {
		q := fmt.Sprintf(`INSERT INTO %s.logistics_products(company_id, name, sort, active)
			VALUES($1,$2,$3,$4)
			RETURNING id, company_id, name, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, cmd.CompanyID, strings.TrimSpace(cmd.Name), cmd.Sort, cmd.Active).Scan(&row.ID, &row.CompanyID, &row.Name, &row.Sort, &row.Active); err != nil {
			return salesapp.LogisticsProduct{}, err
		}
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "logistics_product", &row.ID, "update", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(row.Name), postgresinfra.AuditMeta{"company_id": row.CompanyID, "sort": row.Sort, "active": row.Active})
	return row, nil
}
