package postgres

import (
	"context"
	"fmt"
)

type SchemaStep struct {
	Name string
	Run  func(context.Context) error
}

func EnsureSchema(ctx context.Context, steps []SchemaStep) error {
	for _, step := range steps {
		if step.Run == nil {
			continue
		}
		if err := step.Run(ctx); err != nil {
			if step.Name == "" {
				return err
			}
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return nil
}
