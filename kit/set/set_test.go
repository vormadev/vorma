package set

import "testing"

func TestAddOnNilSet(t *testing.T) {
	var s Set[string]
	s = s.Add("a")
	if !s.Contains("a") {
		t.Fatal("expected set to contain added value")
	}
}

func TestContains(t *testing.T) {
	s := New[int]().Add(1).Add(2)
	if !s.Contains(1) {
		t.Fatal("expected set to contain 1")
	}
	if s.Contains(3) {
		t.Fatal("did not expect set to contain 3")
	}
}
