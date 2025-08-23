package dslflow_test_all

import (
	"context"
	"fmt"
	"github.com/magic-lib/go-plat-utils/conv"
	"github.com/magic-lib/go-plat-utils/templates"
	"github.com/magic-lib/workflow/common/dslflow"
	"testing"
)

func TestActivityMeta(t *testing.T) {

	registerAction()

	act := &dslflow.Activity{
		Id: "act1",
		ActivityMetadata: dslflow.ActivityMetadata{
			Activity: "GetOrderName",
			ArgsForce: map[string]any{
				"name": 5,
			},
			ArgsFallback: map[string]any{
				"age": 7,
			},
			Arguments: "{{id}}",
		},
		Responses: map[string]any{
			"name.ages": "{{name.age}}",
		},
		//Hooks, LifecycleHooks     `yaml:"hooks" json:"hooks,omitempty"`                // activity执行时的钩子程序
		//Timeout           int                `yaml:"timeout" json:"timeout"`           // 超时设置，单位为秒
		//DependsOn         Sequence           `yaml:"depends_on" json:"depends_on"`     // 依赖的服务
		//Cached            bool               `yaml:"cached" json:"cached"`             // 相同的参数请求在整个流程中可以重复使用结果
		//RetryPolicy       *RetryPolicyConfig `yaml:"retry_policy" json:"retry_policy"` // 重试策略
	}

	retData, err := act.Execute(context.Background(), map[string]any{
		"id": 678,
		"name": map[string]any{
			"age": 55,
		},
		"age": 8,
	})

	fmt.Println("result app:")
	fmt.Println(conv.String(retData), err)
}

func TestReplace(t *testing.T) {
	argsMapList := map[string]any{
		"aaa": map[string]any{
			"ccc": 5,
		},
	}
	allParamStrRet := "{{aaa}}{{aaa.ccc}}{{bbb}}"

	tmp := templates.NewTemplate(allParamStrRet)
	allParamStrRet = tmp.Replace(argsMapList)

	fmt.Println(allParamStrRet)
}
func TestReplace22(t *testing.T) {
	argsMapList := map[string]any{
		"name.age1": 5,
		"name": map[string]any{
			"age": 77,
		},
		"name.age": map[string]any{
			"age1": 99,
		},
	}
	allParamStrRet := "{\"name.ages\":\"{{name.age}} + {{name.age1}} + {{name.age.age1}}\"}"

	tmp := templates.NewTemplate(allParamStrRet)
	allParamStrRet = tmp.Replace(argsMapList)

	fmt.Println(allParamStrRet)
}
