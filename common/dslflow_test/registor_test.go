package dslflow_test

import (
	"context"
	"fmt"
	"github.com/magic-lib/go-plat-utils/conv"
	"github.com/magic-lib/workflow/common/dslflow"
	"testing"
)

type Order struct {
	Name string `json:"name"`
}

func (o *Order) GetOrderName(ctx context.Context, id int) (string, error) {
	if id == 0 {
		return "", fmt.Errorf("err:no id")
	}
	fmt.Println("order config:", o.Name)
	return "suc:Order Name", nil
}

func TestActionRegister(t *testing.T) {
	orderModel := &Order{
		Name: "tianlin999",
	}

	actionName := "GetOrderName"

	getOrderNameInterface, err := dslflow.ChangeActionInterface[int, string](orderModel.GetOrderName, &dslflow.ActionMetadata{
		Activity:             actionName,
		RequiredArgumentKeys: []string{"aaa"},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	// 可以放一起进行注册
	actionMap := map[string]dslflow.ActionInterface{
		actionName: getOrderNameInterface,
	}

	for _, v := range actionMap {
		err = dslflow.RegisterAction(v)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	//aa, err := dslflow.GetAction("GetOrderName")
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//data11 := aa.ActionMetadata()
	////fmt.Println(data11, err, "type:", data11.ArgumentType)
	//
	//kk, err := aa.ActionExecute(context.Background(), 0)
	//fmt.Println(kk, err)
	//kk, err = aa.ActionExecute(context.Background(), 5)
	//fmt.Println(kk, err)
	//
	//fmt.Println("\n\n\nexecute:")
	//mm, err := data11.Execute(context.Background(), "0")
	//fmt.Println(mm, err)

}
func TestActivity(t *testing.T) {

	TestActionRegister(t)

	act := &dslflow.Activity{
		Id:       "act1",
		Activity: "GetOrderName",
		ArgumentsForce: map[string]any{
			"name": 5,
		},
		ArgumentsFallback: map[string]any{
			"age": 7,
		},
		Arguments: "{{ 	id  }}",
		Responses: map[string]any{
			"name.age": "{{  name.age  }}",
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
	fmt.Println(conv.String(retData), err)
}
