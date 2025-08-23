package dslflow

import (
	"context"
	"fmt"
	"github.com/magic-lib/go-plat-utils/goroutines"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type (
	Statement struct {
		Control  Control   `yaml:"control" json:"control,omitempty"`   //控制配置
		Activity *Activity `yaml:"activity" json:"activity,omitempty"` //单个活动
		Sequence Sequence  `yaml:"sequence" json:"sequence,omitempty"` //串行情况
		Parallel Parallel  `yaml:"parallel" json:"parallel,omitempty"` //并发情况
	}
)

// Execute 执行单个流程节点（核心流程控制逻辑）
func (s *Statement) Execute(ctx context.Context, vars map[string]any) (map[string]any, error) {
	checked, err := s.Control.checkControlCondition(ctx, vars)
	if err != nil || !checked {
		return vars, err
	}

	activityExcByOrder := s.Control.resolveExecutionOrder(s)
	if len(activityExcByOrder) == 0 {
		return vars, nil
	}

	var resultVars = cloneMap(vars)
	var retErr error
	lo.ForEachWhile(activityExcByOrder, func(orderName OrderType, index int) bool {
		var err error
		var resultVarsTemp map[string]any
		if orderName == activity {
			resultVarsTemp, err = s.Activity.Execute(ctx, resultVars)
		} else if orderName == sequence {
			resultVarsTemp, err = s.Sequence.Execute(ctx, resultVars)
		} else if orderName == parallel {
			resultVarsTemp, err = s.Parallel.Execute(ctx, resultVars)
		}
		if err != nil {
			if s.Control.shouldIgnoreOnError() {
				return true
			}
			retErr = multierr.Append(retErr, fmt.Errorf("activity %s execute failed: %w", orderName, err))
			fmt.Printf("警告：节点执行出错但继续流程: %v\n", err)
			return false
		} else {
			if len(resultVarsTemp) > 0 {
				resultVars = lo.Assign(resultVars, resultVarsTemp)
			}

			if s.Control.shouldExitOnExecute() {
				//后续流程异步执行
				goroutines.GoAsync(func(params ...any) {
					asyncCtx := context.Background()
					var multiErrTemp error
					indexTemp := params[0].(int)
					newVarsTemp := params[1].(map[string]any)
					activityExcByOrderTemp := params[2].([]OrderType)
					for j := indexTemp + 1; j < len(activityExcByOrderTemp); j++ {
						var err error
						var resultVarsTemp map[string]any
						if orderName == activity {
							resultVarsTemp, err = s.Activity.Execute(asyncCtx, newVarsTemp)
						} else if orderName == sequence {
							resultVarsTemp, err = s.Sequence.Execute(asyncCtx, newVarsTemp)
						} else if orderName == parallel {
							resultVarsTemp, err = s.Parallel.Execute(asyncCtx, newVarsTemp)
						}
						if err != nil {
							multiErrTemp = multierr.Append(multiErrTemp, fmt.Errorf("activity %s execute failed: %w", orderName, err))
						} else {
							if len(resultVarsTemp) > 0 {
								newVarsTemp = lo.Assign(newVarsTemp, resultVarsTemp)
							}
						}
					}
				}, index, resultVars, activityExcByOrder)
				return false
			}
		}
		return true
	})

	return resultVars, retErr
}
