package dslflow_test_all

import (
	"testing"
)

type AAAA struct {
	AAA string `json:"aaa"`
	BBB string `json:"bbb"`
}

func TestActionConfig_Execute(t *testing.T) {
	//am := dslflow.ActionMetadata{
	//	Activity:             "test",
	//	ArgumentType:         reflect.TypeOf(&AAAA{}),
	//	RequiredArgumentKeys: []string{"aaa", "ccc.ddd"},
	//}
	//
	//arguments := map[string]any{
	//	"aaa": 11,
	//	"bbb": 22,
	//	"ccc": map[string]any{
	//		"ddd": 33,
	//	},
	//}
	//
	//fmt.Println(err)
}
