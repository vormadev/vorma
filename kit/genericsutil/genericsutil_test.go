package genericsutil

import (
	"testing"
)

type DirectEmptyStruct struct{}
type AliasEmptyStruct = struct{}

func TestNone(t *testing.T) {
	if IsNone(DirectEmptyStruct{}) {
		t.Error("expected false, got true for DirectEmptyStruct{}")
	}
	if IsNone(new(DirectEmptyStruct)) {
		t.Error("expected false, got true for new(DirectEmptyStruct)")
	}
	if !IsNone(AliasEmptyStruct{}) {
		t.Error("expected true, got false for AliasEmptyStruct{}")
	}
	if !IsNone(new(AliasEmptyStruct)) {
		t.Error("expected true, got false for new(AliasEmptyStruct)")
	}
	if !IsNone(None{}) {
		t.Error("expected true, got false for None{}")
	}
	if !IsNone(new(None)) {
		t.Error("expected true, got false for new(None)")
	}
}

type SomeGenericType[I any, O any] struct {
	ZeroHelper[I, O]
}

func TestZeroHelper(t *testing.T) {
	x := SomeGenericType[int, string]{}
	i, o, iPtr, oPtr := x.I(), x.O(), x.IPtr(), x.OPtr()
	if i != 0 {
		t.Errorf("expected 0, got %v", i)
	}
	if o != "" {
		t.Errorf("expected empty string, got %v", o)
	}
	if iPtr == nil {
		t.Error("expected non-nil pointer, got nil")
	}
	if val, ok := iPtr.(*int); !ok || *val != 0 {
		t.Errorf("expected *int, got %T", iPtr)
	}
	if oPtr == nil {
		t.Error("expected non-nil pointer, got nil")
	}
	if val, ok := oPtr.(*string); !ok || *val != "" {
		t.Errorf("expected *string, got %T", oPtr)
	}
	var y any = x
	if _, ok := y.(AnyZeroHelper); !ok {
		t.Error("expected SomeGenericType to implement AnyZeroHelper")
	}
}

func TestZero(t *testing.T) {
	if Zero[int]() != 0 {
		t.Error("expected 0, got non-zero value")
	}
	if Zero[string]() != "" {
		t.Error("expected empty string, got non-empty value")
	}
	if Zero[bool]() != false {
		t.Error("expected false, got true")
	}
}

func TestAnyZeroHelper(t *testing.T) {
	var x AnyZeroHelper = SomeGenericType[int, string]{}
	if x.I() != 0 {
		t.Error("expected 0, got non-zero value")
	}
	if x.O() != "" {
		t.Error("expected empty string, got non-empty value")
	}
	if x.IPtr() == nil {
		t.Error("expected non-nil pointer, got nil")
	}
	if x.OPtr() == nil {
		t.Error("expected non-nil pointer, got nil")
	}
}

func TestAssertOrZero(t *testing.T) {
	var x int = 1
	var y int = 2
	if AssertOrZero[int](x) != 1 {
		t.Error("expected 1, got non-1 value")
	}
	if AssertOrZero[int](y) != 2 {
		t.Error("expected 2, got non-2 value")
	}
	if AssertOrZero[int]("1") != 0 {
		t.Error("expected 0, got non-zero value")
	}
	if AssertOrZero[int]("2") != 0 {
		t.Error("expected 0, got non-zero value")
	}
}
