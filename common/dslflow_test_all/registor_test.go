package dslflow_test_all

import (
	"context"
	"fmt"
	"github.com/agiledragon/gomonkey/v2"
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
	return fmt.Sprintf("add: %d", id+1), nil
}

var ns = ""
var actionName = "GetOrderName"

func registerAction() {
	orderModel := &Order{
		Name: "tianlin999",
	}

	// 1. 创建打桩器
	patch := gomonkey.NewPatches()
	//defer patch.Reset() // 测试结束后恢复原函数，避免影响其他测试

	// 2. 打桩：替换 GetUserID 函数，当输入为 "alice" 时返回 100
	patch.ApplyMethod(orderModel, "GetOrderName", func(o *Order, ctx context.Context, id int) (string, error) {
		fmt.Println("add", id, "success", o.Name)
		return "success", nil
	})

	getOrderNameInterface, err := dslflow.ChangeActionInterface[int, string](orderModel.GetOrderName, &dslflow.ActionMetadata{
		Namespace:            ns,
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
}

func TestActionRegister(t *testing.T) {
	registerAction()

	aa, err := dslflow.GetAction(ns, actionName)
	if err != nil {
		fmt.Println(err)
		return
	}
	data11 := aa.ActionMetadata()
	fmt.Println(conv.String(data11))

	kk, err := aa.ActionExecute(context.Background(), 0)
	fmt.Println(kk, err)
	kk, err = aa.ActionExecute(context.Background(), 5)
	fmt.Println(kk, err)

	fmt.Println("\nexecute:")
	mm, err := data11.Execute(context.Background(), "7")
	fmt.Println(mm, err)

}
