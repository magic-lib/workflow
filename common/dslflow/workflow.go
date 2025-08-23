package dslflow

import (
	"context"
	"fmt"
)

type (
	Workflow struct {
		Variables map[string]any `yaml:"variables" json:"variables,omitempty"` //传入的所有变量参数，包括可以设置某一步的参数
		Root      Statement      `yaml:"root" json:"root,omitempty"`           //启动的根目录
		//Activities []*Activity    `yaml:"activities" json:"activities,omitempty"` //公共的activity资源，用于公共执行的部分,比如公共打日志，可以提高使用率
		Responses map[string]any `yaml:"responses" json:"responses,omitempty"` //请求最终返回的结构
	}
)

// Execute 执行工作流主入口
func (w *Workflow) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	// 1. 初始化全局变量和活动资源池
	globalVars := cloneMap(args)
	if globalVars == nil {
		globalVars = make(map[string]any)
	}
	if len(w.Variables) > 0 {
		//自定义的进行覆盖
		globalVars = jsonPathReplace(args, w.Variables, overridePolicyFallback)
	}

	// 2. 执行根节点流程
	resultVars, err := w.Root.Execute(ctx, globalVars)
	if err != nil {
		return nil, fmt.Errorf("workflow execute failed: %w", err)
	}

	if len(w.Responses) > 0 {
		//映射最终返回结果
		resultVars = jsonPathReplace(resultVars, w.Responses, overridePolicyForce)
	}

	return resultVars, nil
}

//// 映射最终返回结果（根据 Responses 配置提取或转换变量）
//func (w *Workflow) mapFinalResponses(vars map[string]any) map[string]any {
//	if len(w.Responses) == 0 {
//		return vars // 无映射规则时直接返回所有变量
//	}
//
//	finalResult := make(map[string]any)
//	jsonStr := conv.String(vars) // 转为 JSON 便于路径提取
//
//	for targetKey, expr := range w.Responses {
//		exprStr, ok := expr.(string)
//		if ok {
//			// 支持 JSON 路径表达式（如 "user.name" 提取嵌套字段）
//			val := gjson.Get(jsonStr, exprStr)
//			if val.Exists() {
//				finalResult[targetKey] = val.Value()
//			} else {
//				finalResult[targetKey] = nil // 路径不存在时设为 nil
//			}
//		} else {
//			// 直接设置固定值（如常量、默认值）
//			finalResult[targetKey] = expr
//		}
//	}
//	return finalResult
//}
//
//// 执行单个活动节点
//func (s *Statement) executeActivity(ctx context.Context, vars map[string]any, pool activityPool) (map[string]any, error) {
//	// 从资源池获取活动配置（优先使用节点内的配置，其次从公共池查找）
//	activity := s.Activity
//	if activity == nil && s.Activity.Activity != "" {
//		activity = pool[s.Activity.Activity] // 从公共资源池查找
//	}
//	if activity == nil {
//		return vars, errors.New("activity config not found")
//	}
//
//	// 创建活动的上下文（继承全局变量）
//	activityVars := lo.Assign(vars, activity.Arguments) // 合并变量和活动参数
//	activityCtx := ctx
//
//	// 应用活动超时设置
//	if activity.Timeout > 0 {
//		var cancel context.CancelFunc
//		activityCtx, cancel = context.WithTimeout(ctx, activity.Timeout)
//		defer cancel()
//	}
//
//	// 执行活动并获取结果
//	resultMap, err := activity.Execute(activityCtx, newConcurrentMap(activityVars))
//	if err != nil {
//		return vars, fmt.Errorf("activity %s execute failed: %w", activity.Id, err)
//	}
//
//	// 合并活动结果到全局变量
//	return lo.Assign(vars, resultMap.Items()), nil
//}
