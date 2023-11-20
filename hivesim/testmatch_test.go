package hivesim

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tm, err := parseTestPattern("sim/test")
	if err != nil {
		t.Fatal(err)
	}

	if !tm.Match("sim", "test") {
		t.Fatal("expected match")
	}
	if !tm.Match("Sim", "Test") {
		t.Fatal("expected match")
	}
	if !tm.Match("Sim", "TestTest") {
		t.Fatal("expected match")
	}
	if tm.Match("Sim", "Tst") {
		t.Fatal("expected no match")
	}
}
