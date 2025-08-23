package dslflow

import (
	"context"
	"fmt"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type (
	Statement struct {
		Control  Control   `json:"control,omitempty"`  //控制配置
		Activity *Activity `json:"activity,omitempty"` //单个活动
		Parallel Parallel  `json:"parallel,omitempty"` //并发情况
		Sequence Sequence  `json:"sequence,omitempty"` //串行情况
	}
)

// Execute 执行单个流程节点（核心流程控制逻辑）
func (s *Statement) Execute(ctx context.Context, vars map[string]any) (map[string]any, error) {

	// TODO 这里条件已经需要全部满足要求
	checked, err := s.Control.checkControlCondition(vars)
	if err != nil || !checked {
		return vars, err
	}

	activityExcByOrder := s.Control.resolveExecutionOrder(s)
	if len(activityExcByOrder) == 0 {
		return vars, nil
	}

	var resultVars map[string]any
	var retErr error
	lo.ForEachWhile(activityExcByOrder, func(orderName string, index int) bool {
		var err error
		if orderName == activity {
			resultVars, err = s.Activity.Execute(ctx, vars)
		} else if orderName == sequence {
			resultVars, err = s.Sequence.Execute(ctx, vars)
		} else if orderName == parallel {
			resultVars, err = s.Parallel.Execute(ctx, vars)
		}
		if err != nil {
			if s.Control.shouldIgnoreOnError() {
				return true
			}
			retErr = multierr.Append(retErr, fmt.Errorf("activity %s execute failed: %w", orderName, err))
			fmt.Printf("警告：节点执行出错但继续流程: %v\n", err)
			return false
		}
		return true
	})

	return resultVars, retErr
}
