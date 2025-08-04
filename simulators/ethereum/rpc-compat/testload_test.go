package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	data := `// this is a test comment
// this is the second line
// speconly: lalalala
>> {"type":"send"}
<< {"type":"recv"}
`

	expectedComment := `this is a test comment
this is the second line
speconly: lalalala
`
	expectedMessages := []rpcTestMessage{
		{
			data: `{"type":"send"}`,
			send: true,
		},
		{
			data: `{"type":"recv"}`,
			send: false,
		},
	}

	result, err := loadTestFile("the-test", strings.NewReader(data))
	if err != nil {
		t.Fatal("error:", err)
	}
	if result.name != "the-test" {
		t.Error("wrong test name:", result.comment)
	}
	if result.comment != expectedComment {
		t.Errorf("wrong test comment %q", result.comment)
	}
	if !result.speconly {
		t.Error("test is not marked speconly")
	}
	if !reflect.DeepEqual(result.messages, expectedMessages) {
		t.Errorf("wrong test messages %+v", result.messages)
	}
}
