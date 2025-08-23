package dslflow

import (
	"context"
	"fmt"
	"github.com/magic-lib/go-plat-utils/goroutines"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"sync"
)

type (
	// Parallel 并行流程：同时执行多个子节点
	Parallel []*Statement
)

// Execute 执行并行流程（同时执行所有子节点，等待全部完成）
func (p Parallel) Execute(ctx context.Context, vars map[string]any) (map[string]any, error) {
	resultVars := cloneMap(vars)

	// 为每个子节点创建带取消的上下文
	sonCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		multiErr error
	)

	for i, stmt := range p {
		wg.Add(1)
		goroutines.GoAsync(func(params ...any) {
			defer wg.Done()

			currCtx := params[0].(context.Context)
			currStmt := params[1].(*Statement)
			currVars := params[2].(map[string]any)
			currIndex := params[3].(int)

			// 检查上下文是否已取消
			if ctx.Err() != nil {
				mu.Lock()
				multiErr = multierr.Append(multiErr, fmt.Errorf("parallel node %d error: %w", currIndex, ctx.Err()))
				mu.Unlock()
				return
			}

			subVars := cloneMap(currVars)

			res, err := currStmt.Execute(currCtx, subVars)
			if err != nil {
				if stmt.Control.shouldIgnoreOnError() {
					return
				}
				cancel()
				mu.Lock()
				multiErr = multierr.Append(multiErr, fmt.Errorf("parallel node %d error: %w", currIndex, err))
				mu.Unlock()
				return
			}
			// 合并子节点结果
			if len(res) > 0 {
				// 合并子节点结果（加锁保护）
				mu.Lock()
				resultVars = lo.Assign(resultVars, res)
				mu.Unlock()
			}
			return
		}, sonCtx, stmt, vars, i)
	}

	// 等待所有并行节点完成
	wg.Wait()
	return resultVars, multiErr
}
