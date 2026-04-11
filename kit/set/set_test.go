package set

import "testing"

func TestAddOnZeroValue(t *testing.T) {
	var s Set[string]
	s.Add("a")
	if !s.Has("a") {
		t.Fatal("expected set to contain added value")
	}
}

func TestContains(t *testing.T) {
	var s Set[int]
	s.Add(1)
	s.Add(2)
	if !s.Has(1) {
		t.Fatal("expected set to contain 1")
	}
	if s.Has(3) {
		t.Fatal("did not expect set to contain 3")
	}
}

func TestToSlice(t *testing.T) {
	var s Set[string]
	s.Add("a")
	s.Add("b")
	slice := s.Slice()
	expected := map[string]struct{}{
		"a": {},
		"b": {},
	}
	for _, val := range slice {
		if _, ok := expected[val]; !ok {
			t.Fatalf("unexpected value in slice: %s", val)
		}
		delete(expected, val)
	}
	if len(expected) != 0 {
		t.Fatalf("expected values not found in slice: %v", expected)
	}
}
