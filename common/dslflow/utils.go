package dslflow

import (
	"fmt"
	"github.com/magic-lib/go-plat-utils/conv"
	"github.com/magic-lib/go-plat-utils/templates"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	Variables = "variables" //传入的总变量名
	Arguments = "arguments"
	Responses = "responses"
	Result    = "result" //返回值默认的key
)

// 辅助函数：将普通 map 转换为 cmap.ConcurrentMap
func newConcurrentMap(data map[string]any) cmap.ConcurrentMap {
	cm := cmap.New()
	for k, v := range data {
		cm.Set(k, v)
	}
	return cm
}

func cloneMap(data map[string]any) map[string]any {
	m := make(map[string]any)
	err := conv.Unmarshal(conv.String(data), &m)
	if err == nil {
		return m
	}
	return lo.Assign(data)
}
func jsonPathReplace(param map[string]any, data map[string]any, policy OverridePolicy) map[string]any {
	if param == nil {
		param = make(map[string]any)
	}
	if len(data) == 0 {
		return param
	}
	if policy == "" {
		policy = OverridePolicyForce
	}

	jsonStr := conv.String(param)
	var retErr, err error
	for k, v := range data {
		jsonStr, err = jsonPathReplaceOne(jsonStr, k, v, policy)
		if err != nil {
			retErr = err
		}
	}
	if retErr != nil {
		return param
	}

	_ = conv.Unmarshal(jsonStr, &param)

	return param
}

func jsonPathReplaceOne(jsonStr string, jsonPath string, data any, policy OverridePolicy) (string, error) {
	if policy == "" {
		policy = OverridePolicyForce
	}
	if policy == OverridePolicyForce {
		jsonStrTemp, err := sjson.Set(jsonStr, jsonPath, data)
		if err != nil {
			return jsonStr, err
		}
		return jsonStrTemp, nil
	} else if policy == OverridePolicyFallback {
		if !gjson.Get(jsonStr, jsonPath).Exists() {
			jsonStrTemp, err := sjson.Set(jsonStr, jsonPath, data)
			if err != nil {
				return jsonStr, err
			}
			return jsonStrTemp, nil
		}
		return jsonStr, nil
	}

	return jsonStr, fmt.Errorf("无效的覆盖策略: %s", policy)
}

func replaceAllByBindings(args any, bindings map[string]any) (any, error) {
	argsMapList := conv.KeyListFromMap(bindings)
	tmp := templates.NewTemplate(conv.String(args))
	allParamStrRet, err := tmp.Replace(argsMapList)
	if err != nil {
		return args, fmt.Errorf("ReplaceAllByBindings: %w", err)
	}
	_ = conv.Unmarshal(allParamStrRet, args)
	return args, nil
}
