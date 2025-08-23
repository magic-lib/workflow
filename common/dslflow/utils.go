package dslflow

import (
	"fmt"
	"github.com/magic-lib/go-plat-utils/cond"
	"github.com/magic-lib/go-plat-utils/conv"
	"github.com/magic-lib/go-plat-utils/templates"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"reflect"
	"regexp"
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

func createMap(data any) map[string]any {
	m := make(map[string]any)
	_ = conv.Unmarshal(conv.String(data), &m)
	return m
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
		policy = overridePolicyForce
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
		//使用强制策略
		paramMap := conv.KeyListFromMap(param)
		dataMap := conv.KeyListFromMap(data)
		for k, v := range dataMap {
			if policy == overridePolicyForce {
				paramMap[k] = v
			} else if policy == overridePolicyFallback {
				if _, ok := paramMap[k]; !ok {
					paramMap[k] = v
				}
			}
		}
		return conv.MapFromKeyList(paramMap)
	}

	_ = conv.Unmarshal(jsonStr, &param)

	return param
}

func jsonPathReplaceOne(jsonStr string, jsonPath string, data any, policy OverridePolicy) (string, error) {
	if policy == "" {
		policy = overridePolicyForce
	}
	if policy == overridePolicyForce {
		jsonStrTemp, err := sjson.Set(jsonStr, jsonPath, data)
		if err != nil {
			return jsonStr, err
		}
		return jsonStrTemp, nil
	} else if policy == overridePolicyFallback {
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

func replaceAllByBindings(args any, bindings ...map[string]any) (any, error) {
	allParamStrRet := trimTemplateSpaces(conv.String(args))

	var err error
	lo.ForEachWhile(bindings, func(binding map[string]any, _ int) bool {
		argsMapList := conv.KeyListFromMap(binding)
		tmp := templates.NewTemplate(allParamStrRet)
		allParamStrRet = tmp.Replace(argsMapList)
		if err != nil {
			return false
		}
		return true
	})
	if err != nil {
		return args, fmt.Errorf("ReplaceAllByBindings: %w", err)
	}
	if cond.IsPointer(args) {
		_ = conv.Unmarshal(allParamStrRet, args)
		return args, nil
	}

	retInfo, ok := conv.ConvertForType(reflect.TypeOf(args), allParamStrRet)
	if ok {
		return retInfo, nil
	}
	return args, nil
}

// trimTemplateSpaces 去除{{后的空格和}}前的空格
func trimTemplateSpaces(input string) string {
	// 正则表达式解释：
	// {{\s+  匹配{{后面跟一个或多个空白字符
	// (\S.*?) 捕获非空白字符开始的内容（非贪婪模式）
	// \s+}}  匹配一个或多个空白字符后面跟}}
	re := regexp.MustCompile(`{{\s+(\S.*?)\s+}}`)
	return re.ReplaceAllString(input, "{{$1}}")
}
