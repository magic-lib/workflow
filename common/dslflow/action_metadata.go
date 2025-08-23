package dslflow

import (
	"context"
	"errors"
	"fmt"
	"github.com/magic-lib/go-plat-utils/cond"
	"reflect"

	"github.com/magic-lib/go-plat-utils/conv"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

type (
	ActionType string // ActionType 定义动作类型（查询/更新）

	ActionInterface interface {
		ActionExecutor
		ActionMetadata() *ActionMetadata //定义具体的动作属性
	}

	// ActionExecutor 执行Action的接口
	ActionExecutor interface {
		ActionExecute(ctx context.Context, params any) (any, error)
	}
)

const (
	ActionTypeQuery  ActionType = "query"  // 条件执行：查询（无副作用）
	ActionTypeUpdate ActionType = "update" // 动作：数据更新（有副作用）
)

type (
	// ActionMetadata 动作的元数据配置（集中管理所有描述性字段）
	ActionMetadata struct {
		ActionType           ActionType     `yaml:"action_type" json:"action_type"` // 动作类型：query/update
		Namespace            string         `yaml:"namespace" json:"namespace"`
		Activity             string         `yaml:"activity" json:"activity"`                             // 活动名,对应执行的相应方法
		Description          string         `yaml:"description" json:"description"`                       // 动作描述
		RequiredArgumentKeys []string       `yaml:"required_argument_keys" json:"required_argument_keys"` // 必传参数键
		ArgumentType         reflect.Type   `yaml:"-" json:"-"`                                           // 输入参数类型
		Responses            []ReturnConfig `yaml:"responses" json:"responses"`                           // 返回参数元数据
	}

	// ReturnConfig 返回参数元数据（描述返回字段的结构）
	ReturnConfig struct {
		Name        string `yaml:"name" json:"name"`               // 返回字段名称
		Type        string `yaml:"type" json:"type"`               // 字段类型（如 string、int、[]string）
		Required    bool   `yaml:"required" json:"required"`       // 是否必须返回该字段
		Description string `yaml:"description" json:"description"` // 字段描述
	}
)

// 检查输入参数是否符合要求
func (am *ActionMetadata) checkArguments(arguments any) error {
	if am.Activity == "" {
		return errors.New("activity cannot be empty")
	}

	// 检查参数类型是否匹配
	if am.ArgumentType != nil {
		if _, ok := conv.ConvertForType(am.ArgumentType, arguments); !ok {
			return fmt.Errorf("arguments type does not match required type: %v", am.ArgumentType)
		}
	}

	// 检查必填参数
	if len(am.RequiredArgumentKeys) > 0 {
		missingArgs := am.findMissingRequiredArgs(arguments)
		if len(missingArgs) > 0 {
			return fmt.Errorf("missing required arguments: %v", missingArgs)
		}
	}

	return nil
}

// 检查返回数据是否符合要求
func (am *ActionMetadata) checkResponses(retData any) error {
	if len(am.Responses) == 0 {
		return nil
	}

	missingFields := am.findMissingRequiredFields(retData)
	if len(missingFields) > 0 {
		return fmt.Errorf("missing required response fields: %v", missingFields)
	}
	return nil
}

// Execute 执行动作并返回结果
func (am *ActionMetadata) Execute(ctx context.Context, arguments any) (any, error) {
	// 1. 验证输入参数
	if err := am.checkArguments(arguments); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// 2. 获取动作实例
	action, err := GetAction(am.Namespace, am.Activity)
	if err != nil {
		return nil, fmt.Errorf("failed to get action: %w", err)
	}

	// 3. 转换参数并执行动作
	retData, err := am.executeAction(ctx, action, arguments)
	if err != nil {
		return retData, fmt.Errorf("action execution failed: %w", err)
	}

	// 4. 验证返回数据
	if err := am.checkResponses(retData); err != nil {
		return retData, fmt.Errorf("invalid response data: %w", err)
	}

	return retData, nil
}

// 执行动作的内部方法，处理参数转换
func (am *ActionMetadata) executeAction(ctx context.Context, action ActionExecutor, arguments any) (any, error) {
	// 如果指定了参数类型且可以转换，则使用转换后的参数
	if am.ArgumentType != nil {
		if convertedArgs, ok := conv.ConvertForType(am.ArgumentType, arguments); ok {
			return action.ActionExecute(ctx, convertedArgs)
		}
	}

	// 使用原始参数执行
	return action.ActionExecute(ctx, arguments)
}

// 查找缺失的必填参数
func (am *ActionMetadata) findMissingRequiredArgs(arguments any) []string {
	jsonStr := conv.String(arguments)
	if !cond.IsJson(jsonStr) {
		return nil
	}

	missing := make([]string, 0)

	lo.ForEach(am.RequiredArgumentKeys, func(key string, _ int) {
		if !gjson.Get(jsonStr, key).Exists() {
			missing = append(missing, key)
		}
	})

	return missing
}

// 查找缺失的必填返回字段
func (am *ActionMetadata) findMissingRequiredFields(retData any) []string {
	jsonStr := conv.String(retData)
	if !cond.IsJson(jsonStr) {
		return nil
	}

	missing := make([]string, 0)
	lo.ForEach(am.Responses, func(config ReturnConfig, _ int) {
		if config.Name == "" || !config.Required {
			return
		}

		if !gjson.Get(jsonStr, config.Name).Exists() {
			missing = append(missing, config.Name)
		}
	})

	return missing
}
