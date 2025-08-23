package dslflow

import (
	"context"
	"errors"
	"github.com/magic-lib/workflow/common/errorflow"
)

type (
	LifecycleEvent string
	LifecycleHooks map[LifecycleEvent]*Activity
)

const (
	LifecycleEventOnStart    LifecycleEvent = "start"
	LifecycleEventOnComplete LifecycleEvent = "complete"
	LifecycleEventOnSuccess  LifecycleEvent = "success"
	LifecycleEventOnError    LifecycleEvent = "error"
	LifecycleEventOnTimeout  LifecycleEvent = "timeout"
)

// GetHookAction 获取钩子行为
func (lhs LifecycleHooks) getHookAction(e LifecycleEvent) *Activity {
	hook, ok := lhs[e]
	if ok {
		return hook
	}
	return nil
}

// Execute 执行钩子
func (lhs LifecycleHooks) Execute(ctx context.Context, am ActionExecutor, param any) (any, error) {
	_, _ = lhs.executeByEvent(LifecycleEventOnStart)
	retInfo, err := am.ActionExecute(ctx, param)
	if err != nil {
		if errorflow.IsTimeoutError(err) || errors.Is(err, context.DeadlineExceeded) {
			_, _ = lhs.executeByEvent(LifecycleEventOnTimeout)
		} else {
			_, _ = lhs.executeByEvent(LifecycleEventOnError)
		}
	} else {
		_, _ = lhs.executeByEvent(LifecycleEventOnSuccess)
	}
	_, _ = lhs.executeByEvent(LifecycleEventOnComplete)
	return retInfo, err
}

func (lhs LifecycleHooks) executeByEvent(e LifecycleEvent) (any, error) {
	actionRun := lhs.getHookAction(e)
	if actionRun != nil {
		//actionRun.Execute()
		return nil, nil
	}
	return nil, nil
}
