package hivesim

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tm, err := parseTestPattern("sim/test")
	if err != nil {
		t.Fatal(err)
	}

	if !tm.match("sim", "test") {
		t.Fatal("expected match")
	}
	if !tm.match("Sim", "Test") {
		t.Fatal("expected match")
	}
	if !tm.match("Sim", "TestTest") {
		t.Fatal("expected match")
	}
	if tm.match("Sim", "Tst") {
		t.Fatal("expected no match")
	}
}
