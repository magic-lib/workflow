package dslflow

import (
	"context"
	"fmt"
	"github.com/magic-lib/go-plat-cache/cache"
	"github.com/magic-lib/go-plat-utils/conv"
	"github.com/magic-lib/go-plat-utils/crypto"
	"github.com/samber/lo"
	"time"
)

type OverridePolicy string // 默认参数覆盖策略

const (
	OverridePolicyForce    OverridePolicy = "force"    // 强制覆盖
	OverridePolicyFallback OverridePolicy = "fallback" // 缺省覆盖
)

var (
	activityCacheTime = 5 * time.Minute
	activityCache     = cache.NewMemGoCache[any](activityCacheTime, 10*time.Minute)
)

type (
	// Activity 单个 action 配置
	Activity struct {
		Id               string             `yaml:"id" json:"id,omitempty"`                     // 唯一标识，用于区分多个action
		Activity         string             `yaml:"activity" json:"activity,omitempty"`         // 活动名,绑定ActionMetadata里的方法
		DefaultArguments []KeyDefaultConfig `yaml:"default_arguments" json:"default_arguments"` // 按 key 配置的默认参数
		Arguments        map[string]any     `yaml:"arguments" json:"arguments"`                 // 输入参数map，可能有需要转义的字段，所以这里需要设置
		Responses        map[string]any     `yaml:"responses" json:"responses"`                 // 返回的参数map，可以自定义添加内容，比如命名转换
		Hooks            LifecycleHooks     `yaml:"hooks" json:"hooks,omitempty"`               // activity执行时的钩子程序
		Timeout          time.Duration      `yaml:"timeout" json:"timeout"`                     // 超时设置
		DependsOn        *Statement         `yaml:"depends_on" json:"depends_on"`               // 依赖的服务
		Cached           bool               `yaml:"cached" json:"cached"`                       // 相同的参数请求在整个流程中可以重复使用结果
		RetryPolicy      *RetryPolicyConfig `yaml:"retry_policy" json:"retry_policy"`           // 重试策略
	}

	RetryPolicyConfig struct {
		MaximumAttempts int           `yaml:"maximum_attempts" json:"maximum_attempts"` // 最大尝试次数
		InitialInterval time.Duration `yaml:"initial_interval" json:"initial_interval"` // 初始重试间隔
	}

	// KeyDefaultConfig 针对 map 中具体 key 的默认参数配置
	KeyDefaultConfig struct {
		Key            string         `yaml:"key" json:"key"`                         // 目标 key 路径（支持嵌套，如 "user.name"）
		Value          any            `yaml:"value" json:"value"`                     // 默认值
		OverridePolicy OverridePolicy `yaml:"override_policy" json:"override_policy"` // 策略：force/fallback
	}
)

// mergeDefaultArguments 合并默认参数到输入参数
func (ac *Activity) mergeDefaultArguments(args map[string]any) map[string]any {
	if len(ac.DefaultArguments) == 0 || args == nil {
		return args
	}

	// 将参数转为JSON字符串便于路径操作
	jsonStr := conv.String(args)
	isModified := false

	lo.ForEach(ac.DefaultArguments, func(item KeyDefaultConfig, _ int) {
		if item.Key == "" {
			return // 忽略空key配置
		}
		if item.OverridePolicy == "" {
			item.OverridePolicy = OverridePolicyFallback //默认缺省覆盖
		}
		var err error
		jsonStr, err = jsonPathReplaceOne(jsonStr, item.Key, item.Value, item.OverridePolicy)
		if err != nil {
			fmt.Printf("警告：合并参数 key=%s 失败: %v\n", item.Key, err)
		} else {
			isModified = true
		}
	})

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
		retMap[ac.Activity] = retData
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
	inputParams := cloneMap(args)

	// 1. 合并默认参数到输入参数
	inputParams = ac.mergeDefaultArguments(inputParams)

	execCtx := ctx
	if ac.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, ac.Timeout)
		defer cancel()
	}

	// 2. 执行依赖动作并合并结果
	depParams, err := ac.exeDependsOn(execCtx, inputParams)
	if err != nil {
		return inputParams, fmt.Errorf("依赖执行失败: %w", err)
	}

	// TODO 3. 合并参数
	if len(ac.Arguments) > 0 {
		depParams = jsonPathReplace(depParams, ac.Arguments, OverridePolicyForce)
	}

	// 4. 执行主动作
	execOneAction := func(ctx context.Context, param any) (any, error) {
		actIns, err := GetAction(ac.Activity)
		if err != nil {
			return nil, fmt.Errorf("获取动作实例失败: %w", err)
		}

		paramKey := ""

		if ac.Cached {
			//是否有缓存
			paramKey = crypto.Md5(conv.String(param))
			actionResult, err := cache.NsGet[any](ctx, activityCache, ac.Activity, paramKey)
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
			_, _ = cache.NsSet[any](ctx, activityCache, ac.Activity, paramKey, actionResult, activityCacheTime)
		}

		return actionResult, nil
	}
	retData, err := ac.executeWithRetry(execOneAction, execCtx, depParams)

	if err != nil {
		return depParams, fmt.Errorf("合并结果失败: %w", err)
	}

	if len(ac.Responses) > 0 {
		retData = jsonPathReplace(retData, ac.Responses, OverridePolicyForce)
	}

	return retData, nil
}

// exeDependsOn 执行依赖的前置动作并合并结果
func (ac *Activity) exeDependsOn(ctx context.Context, inputParams map[string]any) (map[string]any, error) {
	if ac.DependsOn == nil {
		return inputParams, nil
	}

	return ac.DependsOn.Execute(ctx, inputParams)
}
