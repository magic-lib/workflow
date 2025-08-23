package dslflow

import (
	"context"
	"fmt"
	"github.com/magic-lib/go-plat-cache/cache"
	"github.com/magic-lib/go-plat-utils/conv"
	"github.com/magic-lib/go-plat-utils/crypto"
	"github.com/magic-lib/go-plat-utils/templates"
	"github.com/samber/lo"
	"time"
)

type OverridePolicy string // 默认参数覆盖策略

const (
	overridePolicyForce    OverridePolicy = "force"    // 强制覆盖
	overridePolicyFallback OverridePolicy = "fallback" // 缺省覆盖
)

var (
	activityCacheTime = 5 * time.Minute
	activityCache     = cache.NewMemGoCache[any](activityCacheTime, 10*time.Minute)
)

type (
	ActivityMetadata struct {
		Namespace    string         `yaml:"namespace" json:"namespace"`
		Activity     string         `yaml:"activity" json:"activity,omitempty"`
		ArgsForce    map[string]any `yaml:"args_force" json:"args_force"`       // 按 key 配置的默认参数强制覆盖
		ArgsFallback map[string]any `yaml:"args_fallback" json:"args_fallback"` // 按 key 配置的默认参数缺省覆盖
		Arguments    string         `yaml:"arguments" json:"arguments"`         // 定义调用action的特殊参数，比如action参数是数字，则在这里进行转换，支持输入模版 {{name.id}}
	}

	// Activity 单个 action 配置
	Activity struct {
		ActivityMetadata
		Id          string            `yaml:"id" json:"id,omitempty"`           // 唯一标识，用于区分多个action
		Responses   map[string]any    `yaml:"responses" json:"responses"`       // 返回的参数map，可以自定义添加内容，比如命名转换
		Hooks       LifecycleHooks    `yaml:"hooks" json:"hooks,omitempty"`     // activity执行时的钩子程序
		Timeout     int               `yaml:"timeout" json:"timeout"`           // 超时设置，单位为秒
		DependsOn   any               `yaml:"depends_on" json:"depends_on"`     // 依赖的服务
		Cached      bool              `yaml:"cached" json:"cached"`             // 相同的参数请求在整个流程中可以重复使用结果
		RetryPolicy RetryPolicyConfig `yaml:"retry_policy" json:"retry_policy"` // 重试策略
	}

	RetryPolicyConfig struct {
		MaximumAttempts int           `yaml:"maximum_attempts" json:"maximum_attempts"` // 最大尝试次数
		InitialInterval time.Duration `yaml:"initial_interval" json:"initial_interval"` // 初始重试间隔
	}
)

// mergeDefaultArguments 合并默认参数到输入参数
func (ac ActivityMetadata) mergeDefaultArguments(args map[string]any) map[string]any {
	if (len(ac.ArgsForce) == 0 && len(ac.ArgsFallback) == 0) || args == nil {
		return args
	}

	// 将参数转为JSON字符串便于路径操作
	jsonStr := conv.String(args)
	isModified := false

	if len(ac.ArgsFallback) > 0 {
		for k, v := range ac.ArgsFallback {
			var err error
			jsonStrTemp, err := jsonPathReplaceOne(jsonStr, k, v, overridePolicyFallback)
			if err != nil {
				fmt.Printf("警告：合并参数 key=%s 失败: %v\n", k, err)
			} else {
				jsonStr = jsonStrTemp
				isModified = true
			}
		}
	}

	if len(ac.ArgsForce) > 0 {
		for k, v := range ac.ArgsForce {
			var err error
			jsonStrTemp, err := jsonPathReplaceOne(jsonStr, k, v, overridePolicyForce)
			if err != nil {
				fmt.Printf("警告：合并参数 key=%s 失败: %v\n", k, err)
			} else {
				jsonStr = jsonStrTemp
				isModified = true
			}
		}
	}

	// 如果参数有修改，反序列化回map
	if isModified {
		newArgs := make(map[string]any)
		if err := conv.Unmarshal(jsonStr, &newArgs); err != nil {
			fmt.Printf("警告：参数反序列化失败: %v\n", err)
		} else {
			return newArgs
		}
	}

	return args
}

// executeWithRetry 带重试机制执行函数，至少需要执行一次
func (ac *Activity) executeWithRetry(fn ActionMethod, ctx context.Context, arguments any) (map[string]any, error) {
	maxAttempts := ac.RetryPolicy.MaximumAttempts //这个是重试的次数
	if maxAttempts <= 0 {
		maxAttempts = 0
	}
	maxAttempts = maxAttempts + 1 // 最少执行一次

	initialInterval := ac.RetryPolicy.InitialInterval
	if initialInterval <= 0 {
		initialInterval = 50 * time.Millisecond
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if retData, err := fn(ctx, arguments); err != nil {
			lastErr = err
			if attempt < maxAttempts {
				// 指数退避重试（间隔翻倍）
				backoff := initialInterval * time.Duration(1<<(attempt-1))
				fmt.Printf("动作执行失败（尝试 %d/%d），%v后重试: %v\n", attempt, maxAttempts, backoff, err)
				time.Sleep(backoff)
			}
		} else {
			//fmt.Printf("action %s, ret: %s", ac.Activity, conv.String(retData))

			return ac.createResponse(arguments, retData), nil // 成功执行
		}
	}
	return nil, lastErr // 返回最后一次错误
}

// createResponse 生成返回的结果
func (ac *Activity) createResponse(requestParams any, retData any) map[string]any {
	resultMap := make(map[string]any)

	paramMap := make(map[string]any)
	_ = conv.Unmarshal(requestParams, &paramMap)
	if len(paramMap) > 0 {
		resultMap = lo.Assign(resultMap, paramMap)
	}

	retMap := make(map[string]any)
	_ = conv.Unmarshal(retData, &retMap)
	if len(retMap) == 0 {
		actionKey := getActionKey(ac.Namespace, ac.Activity)
		retMap[actionKey] = retData
	}
	resultMap = lo.Assign(resultMap, retMap)

	if ac.Id != "" {
		resultMap = lo.Assign(resultMap, map[string]any{
			ac.Id: map[string]any{
				Arguments: requestParams,
				Responses: retData,
			},
		})
	}
	return resultMap
}

// Execute 执行动作主逻辑：合并参数→执行依赖→执行主动作→合并结果
func (ac *Activity) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	inputParams := ac.makeInputMap(args)

	// 0、获取当前活动的所有参数
	execCtx := ctx
	if ac.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(ac.Timeout)*time.Second)
		defer cancel()
	}

	// 2. 执行依赖动作并合并结果
	depParams, err := ac.exeDependsOn(execCtx, inputParams)
	if err != nil {
		return inputParams, fmt.Errorf("依赖执行失败: %w", err)
	}

	// 3. 合并生成执行参数
	var actionParam any = depParams
	if ac.Arguments != "" {
		actionParam, err = replaceAllByBindings(ac.Arguments, depParams)
		if err != nil {
			return inputParams, fmt.Errorf("参数替换失败: %w", err)
		}
	}

	// 4. 执行主动作
	execOneAction := func(ctx context.Context, param any) (any, error) {
		actionKey := getActionKey(ac.Namespace, ac.Activity)
		actIns, err := GetAction(ac.Namespace, ac.Activity)
		if err != nil {
			return nil, fmt.Errorf("获取动作实例失败: %w", err)
		}

		paramKey := ""

		if ac.Cached {
			//是否有缓存
			paramKey = crypto.Md5(conv.String(param))
			actionResult, err := cache.NsGet[any](ctx, activityCache, actionKey, paramKey)
			if err == nil {
				return actionResult, nil
			}
		}

		var actionResult any
		var execErr error
		if ac.Hooks != nil {
			actionResult, execErr = ac.Hooks.Execute(execCtx, actIns, param)
		} else {
			actionResult, execErr = actIns.ActionExecute(ctx, param)
		}

		if execErr != nil {
			return nil, fmt.Errorf("主动作执行失败: %w", execErr)
		}

		// 需要缓存该执行对象
		if ac.Cached && paramKey != "" {
			_, _ = cache.NsSet[any](ctx, activityCache, actionKey, paramKey, actionResult, activityCacheTime)
		}

		return actionResult, nil
	}
	retData, err := ac.executeWithRetry(execOneAction, execCtx, actionParam)

	if err != nil {
		return depParams, fmt.Errorf("合并结果失败: %w", err)
	}

	//将所有参数合并进所有的对象中
	overrideParams := []map[string]any{inputParams, depParams, args, retData}
	if len(ac.Responses) > 0 {
		acResponses, err := replaceAllByBindings(ac.Responses, overrideParams...)
		if err != nil {
			return nil, fmt.Errorf("参数替换失败1: %w", err)
		}

		acResponseMap := createMap(acResponses)
		newRetData := jsonPathReplace(retData, acResponseMap, overridePolicyForce)
		// args 初始参数  depParams 合并depends以后的参数  retData 执行后返回的结果
		actionReturn, err := replaceAllByBindings(&newRetData, overrideParams...)
		if err != nil {
			return nil, fmt.Errorf("参数替换失败2: %w", err)
		}
		actionReturnStr := conv.String(actionReturn)
		// 将变量重新替换为真实值
		tmp := templates.NewTemplate(actionReturnStr)
		actionReturnStr = tmp.Replace(actionReturn)

		actionReturnMap := make(map[string]any)
		_ = conv.Unmarshal(actionReturnStr, &actionReturnMap)
		allParam := lo.Assign(overrideParams...)
		return lo.Assign(allParam, actionReturnMap), nil
	}

	return lo.Assign(overrideParams...), nil
}

// exeDependsOn 执行依赖的前置动作并合并结果
func (ac *Activity) exeDependsOn(ctx context.Context, inputParams map[string]any) (map[string]any, error) {
	switch depType := ac.DependsOn.(type) {
	case []ActivityMetadata:
		// 新逻辑：处理名称列表
		return ac.executeDependsByName(ctx, inputParams, depType)
	case Sequence:
		// 旧逻辑：处理原有Sequence
		return depType.Execute(ctx, inputParams)
	case nil:
		return inputParams, nil
	default:
		return nil, fmt.Errorf("不支持的依赖类型：%T", depType)
	}
}

func (ac *Activity) executeDependsByName(ctx context.Context, inputParams map[string]any, deptNames []ActivityMetadata) (map[string]any, error) {
	if len(deptNames) == 0 {
		return inputParams, nil
	}

	mergedParams := cloneMap(inputParams)

	return mergedParams, nil
}

func (ac *Activity) makeInputMap(arguments map[string]any) map[string]any {
	args := cloneMap(arguments)

	//2、将是自己id的参数覆盖进来
	if ac.Id != "" {
		if oneParam, ok := args[ac.Id]; ok {
			if oneParamMap, ok1 := oneParam.(map[string]any); ok1 {
				for k, v := range oneParamMap {
					args[k] = v
				}
			}
		}
	}

	// 本身的参数列表是否包含
	//3、activity中自定义进行覆盖，主要是将前面流程的参数和返回值加到里面
	args = ac.mergeDefaultArguments(args)

	return args
}
