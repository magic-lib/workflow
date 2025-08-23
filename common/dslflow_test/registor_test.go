package dslflow_test

import (
	"context"
	"fmt"
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

	for k, v := range actionMap {
		fmt.Println(k)
		err := dslflow.RegisterAction(v)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	aa, err := dslflow.GetAction("GetOrderName")
	if err != nil {
		fmt.Println(err)
		return
	}
	data11 := aa.ActionMetadata()
	fmt.Println(data11, err, "type:", data11.ArgumentType)

	kk, err := aa.ActionExecute(context.Background(), 0)
	fmt.Println(kk, err)
	kk, err = aa.ActionExecute(context.Background(), 5)
	fmt.Println(kk, err)

	fmt.Println("execute:")
	mm, err := data11.Execute(context.Background(), 5)
	fmt.Println(mm, err)

}
