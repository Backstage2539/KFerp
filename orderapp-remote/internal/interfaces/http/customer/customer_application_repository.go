package customer

import (
	"context"
	"os"
	"path/filepath"

	customerapp "orderapp/internal/application/customer"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresCustomerApplicationRepository struct {
	pool     *pgxpool.Pool
	schema   string
	assetDir string
}

func (r postgresCustomerApplicationRepository) Upsert(ctx context.Context, actor string, id *int64, cmd customerapp.UpsertCommand) (int64, error) {
	req := CustomerUpsertRequest{
		Name:               cmd.Name,
		RawName:            cmd.RawName,
		Contact:            cmd.Contact,
		Phone:              cmd.Phone,
		Address:            cmd.Address,
		DefaultSourceID:    cmd.DefaultSourceID,
		DefaultOrderTypeID: cmd.DefaultOrderTypeID,
		Active:             cmd.Active,
	}
	return upsertCustomer(ctx, r.pool, r.schema, actor, id, &req)
}

func (r postgresCustomerApplicationRepository) Prefs(ctx context.Context, id int64) (*customerapp.Prefs, error) {
	p, err := fetchCustomerPrefs(ctx, r.pool, r.schema, id)
	if err != nil {
		return nil, err
	}
	return &customerapp.Prefs{
		ID:              p.ID,
		DefaultSourceID: p.DefaultSourceID,
		SourceName:      p.SourceName,
		DefaultTypeID:   p.DefaultTypeID,
		TypeName:        p.TypeName,
		Address:         p.Address,
	}, nil
}

func (r postgresCustomerApplicationRepository) SaveAsset(ctx context.Context, cmd customerapp.SaveAssetCommand) (customerapp.SaveAssetResult, error) {
	obj, size, sha, err := saveCustomerAssetFile(r.assetDir, cmd.CustomerID, cmd.Kind, cmd.Reader, cmd.ContentType, cmd.MaxBytes, cmd.Filename)
	if err != nil {
		return customerapp.SaveAssetResult{}, err
	}
	if _, err := insertCustomerAsset(ctx, r.pool, r.schema, cmd.Actor, cmd.CustomerID, cmd.Kind, obj, cmd.ContentType, size, sha); err != nil {
		return customerapp.SaveAssetResult{}, err
	}
	return customerapp.SaveAssetResult{CustomerID: cmd.CustomerID, ObjectKey: obj, Bytes: size, SHA256: sha}, nil
}

func (r postgresCustomerApplicationRepository) DeleteAsset(ctx context.Context, actor string, assetID int64) (customerapp.DeleteAssetResult, error) {
	cid, _, obj, err := deleteCustomerAssetByID(ctx, r.pool, r.schema, actor, assetID)
	if err != nil {
		return customerapp.DeleteAssetResult{}, err
	}
	if obj != "" {
		_ = os.Remove(filepath.Join(r.assetDir, obj))
	}
	return customerapp.DeleteAssetResult{CustomerID: cid, ObjectKey: obj}, nil
}

func (r postgresCustomerApplicationRepository) InlineUpdate(ctx context.Context, actor string, id int64, cmd customerapp.InlineUpdateCommand) error {
	req := CustomerInlineReq{
		Name:               cmd.Name,
		Contact:            cmd.Contact,
		Phone:              cmd.Phone,
		Address:            cmd.Address,
		DefaultSourceID:    cmd.DefaultSourceID,
		DefaultOrderTypeID: cmd.DefaultOrderTypeID,
		Active:             cmd.Active,
	}
	return inlineUpdateCustomer(ctx, r.pool, r.schema, actor, id, &req)
}

func (r postgresCustomerApplicationRepository) Delete(ctx context.Context, actor string, id int64) error {
	return deleteCustomer(ctx, r.pool, r.schema, actor, id)
}
