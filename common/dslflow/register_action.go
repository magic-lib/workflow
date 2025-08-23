package dslflow

import (
	"context"
	"fmt"
	cmapv2 "github.com/orcaman/concurrent-map/v2"
	"reflect"
	"runtime"
)

type (
	ActionMethod func(ctx context.Context, param any) (any, error)
)

var (
	actionRegistry = cmapv2.New[ActionInterface]()
)

type methodAdapter struct {
	method     ActionMethod //action执行的具体方法
	actionMeta *ActionMetadata
}

func (m *methodAdapter) ActionExecute(ctx context.Context, params any) (any, error) {
	return m.method(ctx, params)
}
func (m *methodAdapter) ActionMetadata() *ActionMetadata {
	return m.actionMeta
}

// changeActionMethod 将普通方法转换为ActionMethod
func changeActionMethod[I, O any](method any) (ActionMethod, error) {
	if method == nil {
		return nil, fmt.Errorf("method is nil")
	}
	methodFun, ok := method.(func(ctx context.Context, param I) (O, error))
	if !ok {
		methodName := ""
		pc := reflect.ValueOf(method).Pointer()
		funcInfo := runtime.FuncForPC(pc)
		if funcInfo != nil {
			methodName = funcInfo.Name()
		}
		return nil, fmt.Errorf("method is not func(ctx context.Context, param I) (O, error): %s", methodName)
	}
	return func(ctx context.Context, param any) (retData any, err error) {
		//断言
		paramPtr, ok := param.(I)
		if !ok {
			var zero I
			return nil, fmt.Errorf("param is not %T, not %T", paramPtr, reflect.TypeOf(zero).Name())
		}
		retData, err = methodFun(ctx, paramPtr)
		retDataPtr, ok := retData.(O)
		if !ok {
			var zero O
			return nil, fmt.Errorf("retData is %T, not %T", retDataPtr, reflect.TypeOf(zero).Name())
		}
		//调用方法
		return retData, err
	}, nil
}

// ChangeActionInterface 转换为ActionInterface
func ChangeActionInterface[I, O any](method any, ac *ActionMetadata) (ActionInterface, error) {
	if ac == nil || method == nil {
		return nil, fmt.Errorf("data or method is nil")
	}
	if ac.Activity == "" {
		return nil, fmt.Errorf("activity name is empty")
	}
	if ac.ActionType == "" {
		ac.ActionType = ActionTypeQuery
	}

	am, err := changeActionMethod[I, O](method)
	if err != nil {
		return nil, err
	}
	if ac.ArgumentType == nil {
		var zero I
		ac.ArgumentType = reflect.TypeOf(zero)
	}

	return &methodAdapter{
		actionMeta: ac,
		method:     am,
	}, nil
}

// RegisterAction 注册全局Action方法
func RegisterAction(ai ActionInterface) error {
	if ai == nil {
		return fmt.Errorf("ai is nil")
	}
	am := ai.ActionMetadata()
	if am == nil || am.Activity == "" {
		return fmt.Errorf("activity name is empty")
	}
	// 不能重复注册，避免覆盖
	if actionRegistry.Has(am.Activity) {
		return fmt.Errorf("activity %s is already registered", am.Activity)
	}
	actionRegistry.Set(am.Activity, ai)
	return nil
}

// GetAction 获取Action方法
func GetAction(activity string) (ActionInterface, error) {
	ai, ok := actionRegistry.Get(activity)
	if !ok {
		return nil, fmt.Errorf("activity %s is not registered", activity)
	}
	return ai, nil
}
