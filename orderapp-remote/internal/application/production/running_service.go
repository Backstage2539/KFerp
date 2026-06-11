package production

import (
	"context"
	"fmt"
)

func startNeedKey(productID, specG int64) string {
	return fmt.Sprintf("%d-%d", productID, specG)
}

func (s *Service) Start(ctx context.Context, cmd StartCommand) (StartResult, error) {
	needs, err := s.repo.ListStartNeeds(ctx, cmd)
	if err != nil {
		return StartResult{}, err
	}
	plan := make([]StartNeed, 0)
	for _, need := range needs {
		if need.GapG <= 0 {
			continue
		}
		if cmd.Selected[startNeedKey(need.ProductID, need.SpecG)] {
			plan = append(plan, need)
		}
	}
	if len(plan) == 0 {
		return StartResult{}, fmt.Errorf("没有可开始生产的数据")
	}
	if cmd.InputByKey == nil {
		cmd.InputByKey = map[string]int64{}
	}
	return s.repo.Start(ctx, StartExecutionCommand{
		Needs:      plan,
		InputByKey: cmd.InputByKey,
		Operator:   cmd.Operator,
	})
}
