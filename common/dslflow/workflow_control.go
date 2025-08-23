package dslflow

import (
	"context"
	"fmt"
	"github.com/magic-lib/go-plat-utils/templates"
	"github.com/magic-lib/go-plat-utils/templates/ruleengine"
	"github.com/samber/lo"
	"sort"
)

const (
	OnErrorIgnore OnErrorType = "ignore" //遇到错误时，忽略错误，继续执行，比如打日志，或者是发消息，错误就错误了

	OnExitExit OnExitType = "exit"
)

type (
	OrderType   string
	OnExitType  string
	OnErrorType string
	Control     struct {
		When           string      `yaml:"when" json:"when,omitempty"`                       //执行的前提条件
		DependsOn      []*Activity `yaml:"depends_on" json:"depends_on,omitempty"`           //新增依赖声明，主要是针对When中包含的依赖
		OnError        OnErrorType `yaml:"onerror" json:"onerror,omitempty"`                 //如果出现执行错误了，是直接跳出，还是继续执行
		OnExit         OnExitType  `yaml:"onexit" json:"onexit,omitempty"`                   //是否执行完当前的Activity后，就直接返回，后续的则子流程
		Wait           string      `yaml:"wait" json:"wait,omitempty"`                       //是否需要有等待的Channel
		ExecutionOrder []OrderType `yaml:"execution_order" json:"execution_order,omitempty"` //执行顺序
	}
)

const (
	activity OrderType = "activity"
	sequence OrderType = "sequence"
	parallel OrderType = "parallel"
)

// 检查控制条件是否满足（简化实现，实际可集成表达式引擎）
func (c *Control) checkControlCondition(ctx context.Context, vars map[string]any) (bool, error) {
	if c.When == "" {
		return true, nil // 无条件时默认执行
	}

	retAllMap, err := c.executeDependsOn(ctx, vars)
	if err != nil {
		return false, fmt.Errorf("执行依赖失败: %w", err)
	}

	//首先替换掉变量
	tmp := templates.NewTemplate(c.When)
	allParamWhen := tmp.Replace(retAllMap)

	fmt.Println("checkControlCondition:", allParamWhen)

	ruleEngine := ruleengine.NewEngineLogic()
	retCheck, err := ruleEngine.RunString(allParamWhen, vars)
	if err != nil {
		return false, fmt.Errorf("条件解析失败: %w", err)
	}
	if retBool, ok := retCheck.(bool); ok {
		return retBool, nil
	}
	return false, fmt.Errorf("条件解析非bool类型")
}

// 检查控制条件是否满足（简化实现，实际可集成表达式引擎）
func (c *Control) executeDependsOn(ctx context.Context, vars map[string]any) (map[string]any, error) {
	if c.When == "" || len(c.DependsOn) == 0 {
		return vars, nil // 无条件时默认执行
	}

	allVars := vars
	var retErr error
	lo.ForEach(c.DependsOn, func(activity *Activity, index int) {
		newVars, err := activity.Execute(ctx, allVars)
		if err != nil {
			retErr = fmt.Errorf("activity %s execute failed: %w", activity.Id, err)
			return
		}
		for k, v := range newVars {
			allVars[k] = v
		}
	})
	if retErr != nil {
		return allVars, retErr
	}
	return allVars, nil
}

// 解析执行顺序，返回优先级最高的字段类型
func (c *Control) resolveExecutionOrder(stmt *Statement) []OrderType {
	// 1. 定义所有可能的元素及其默认优先级（数字越小优先级越高）
	defaultOrder := map[OrderType]int{
		activity: 1,
		sequence: 2,
		parallel: 3,
	}

	// 2. 处理用户配置的 ExecutionOrder，覆盖默认优先级
	configOrder := make(map[OrderType]int)
	if len(c.ExecutionOrder) > 0 {
		for idx, item := range c.ExecutionOrder {
			// 只处理合法值（过滤无效配置）
			if _, ok := defaultOrder[item]; ok {
				configOrder[item] = idx + 1 // 配置中的索引越小，优先级越高（从1开始）
			}
		}
	}

	// 3. 收集当前 Statement 中已配置的字段
	availableItems := make([]OrderType, 0)
	if stmt.Activity != nil {
		availableItems = append(availableItems, activity)
	}
	if len(stmt.Sequence) > 0 {
		availableItems = append(availableItems, sequence)
	}
	if len(stmt.Parallel) > 0 {
		availableItems = append(availableItems, parallel)
	}

	// 4. 找到优先级最高的字段（优先级数值最小）
	if len(availableItems) == 0 {
		return nil
	}

	// 4. 按优先级对可用字段排序（升序：优先级数值越小越靠前）
	// 使用 sort.Slice 自定义排序规则
	sort.Slice(availableItems, func(i, j int) bool {
		itemI := availableItems[i]
		itemJ := availableItems[j]

		// 获取 i 和 j 的优先级（优先用配置，无配置则用默认）
		priorityI := c.getPriority(itemI, configOrder, defaultOrder)
		priorityJ := c.getPriority(itemJ, configOrder, defaultOrder)

		// 优先级数值小的排在前面
		return priorityI < priorityJ
	})

	return availableItems
}

// 获取字段的最终优先级（优先使用配置，无配置则用默认）
func (c *Control) getPriority(item OrderType, configOrder, defaultOrder map[OrderType]int) int {
	if p, ok := configOrder[item]; ok {
		return p
	}
	return defaultOrder[item]
}

// 判断是否在执行后立即退出
func (c *Control) shouldExitOnExecute() bool {
	if c == nil {
		return false // 默认不退出
	}
	return c.OnExit == OnExitExit
}
func (c *Control) shouldIgnoreOnError() bool {
	if c == nil {
		return false
	}
	return c.OnError == OnErrorIgnore
}
