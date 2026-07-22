package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadLargeMessage(t *testing.T) {
	const messageSize = 24 * 1024 * 1024
	data := ">> {\"payload\":\"" + strings.Repeat("a", messageSize) + "\"}\n"

	result, err := loadTestFile("large-message", strings.NewReader(data))
	if err != nil {
		t.Fatal("error:", err)
	}
	if len(result.messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(result.messages))
	}
	if !result.messages[0].send {
		t.Fatal("large message was not parsed as a request")
	}
}

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
