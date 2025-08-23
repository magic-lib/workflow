package dslflow

import (
	"context"
	"github.com/magic-lib/go-plat-utils/goroutines"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type (
	// Sequence 串行流程：按顺序执行多个子节点
	Sequence []*Statement
)

// Execute 执行串行流程（按顺序执行每个子节点）
func (seq Sequence) Execute(ctx context.Context, vars map[string]any) (map[string]any, error) {
	// 需要复制参数列表，防止出现并发修改的问题
	newVars := cloneMap(vars)

	sonCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 前面执行的结果，可能成为后面的参数
	var multiErr error
	for i, stmt := range seq {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return newVars, ctx.Err()
		}

		// 执行子节点
		resultVars, err := stmt.Execute(sonCtx, newVars)
		if err != nil {
			if stmt.Control.shouldIgnoreOnError() {
				continue
			}
			multiErr = multierr.Append(multiErr, err)
			break
		}
		// 合并子节点结果
		if len(resultVars) > 0 {
			newVars = lo.Assign(newVars, resultVars)
		}
		if stmt.Control.shouldExitOnExecute() {
			//后续流程异步执行
			goroutines.GoAsync(func(params ...any) {
				asyncCtx := context.Background()
				var multiErrTemp error
				index := params[0].(int)
				newVarsTemp := params[1].(map[string]any)
				for j := index + 1; j < len(seq); j++ {
					resultVarsTemp, err := seq[j].Execute(asyncCtx, newVarsTemp)
					if err != nil {
						multiErrTemp = multierr.Append(multiErrTemp, err)
						break
					}
					// 合并子节点结果
					if len(resultVarsTemp) > 0 {
						newVarsTemp = lo.Assign(newVarsTemp, resultVarsTemp)
					}
				}
			}, i, newVars)
			break
		}
	}

	return newVars, multiErr
}
