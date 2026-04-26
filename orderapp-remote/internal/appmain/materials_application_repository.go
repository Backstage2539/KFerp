package appmain

import (
	"context"

	materialsapp "orderapp/internal/application/materials"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresMaterialsRepository struct {
	pool   *pgxpool.Pool
	schema string
}

func (r postgresMaterialsRepository) List(ctx context.Context, cmd materialsapp.ListCommand) ([]materialsapp.Material, error) {
	rows, err := listMaterials(ctx, r.pool, r.schema, cmd.Query, cmd.Limit)
	if err != nil {
		return nil, err
	}
	return materialsToApp(rows), nil
}

func (r postgresMaterialsRepository) Update(ctx context.Context, cmd materialsapp.UpdateCommand) (materialsapp.Material, error) {
	row, err := updateMaterialInline(ctx, r.pool, r.schema, cmd.Actor, cmd.ID, materialInputFromApp(cmd.Input))
	if err != nil {
		return materialsapp.Material{}, err
	}
	return materialToApp(row), nil
}

func materialsToApp(rows []MaterialRow) []materialsapp.Material {
	out := make([]materialsapp.Material, 0, len(rows))
	for _, row := range rows {
		out = append(out, materialToApp(row))
	}
	return out
}

func materialToApp(row MaterialRow) materialsapp.Material {
	return materialsapp.Material{
		ID:            row.ID,
		Code:          row.Code,
		Name:          row.Name,
		Kind:          row.Kind,
		Unit:          row.Unit,
		PurchasePrice: row.PurchasePrice,
		SalePrice:     row.SalePrice,
		OnhandG:       row.OnhandG,
		OnhandUnits:   row.OnhandUnits,
		MinLevelG:     row.MinLevelG,
		MinLevelUnits: row.MinLevelUnits,
		UpdatedAt:     row.UpdatedAt,
	}
}

func materialInputFromApp(in materialsapp.MaterialInput) MaterialInput {
	return MaterialInput{
		Code:          in.Code,
		Name:          in.Name,
		Kind:          in.Kind,
		Unit:          in.Unit,
		PurchasePrice: in.PurchasePrice,
		SalePrice:     in.SalePrice,
		OnhandG:       in.OnhandG,
		OnhandUnits:   in.OnhandUnits,
		MinLevelG:     in.MinLevelG,
		MinLevelUnits: in.MinLevelUnits,
	}
}
