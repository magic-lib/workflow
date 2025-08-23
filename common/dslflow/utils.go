package dslflow

import (
	"github.com/magic-lib/go-plat-utils/conv"
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
	var err error
	for k, v := range data {
		if policy == OverridePolicyForce {
			jsonStr, err = sjson.Set(jsonStr, k, v)
			continue
		} else if policy == OverridePolicyFallback {
			if !gjson.Get(jsonStr, k).Exists() {
				jsonStr, err = sjson.Set(jsonStr, k, v)
			}
			continue
		}
	}
	if err != nil {
		return param
	}

	_ = conv.Unmarshal(jsonStr, &param)

	return param
}
