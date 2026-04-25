package enum

import "testing"

type trade_side string

func TestNew(t *testing.T) {
	trade_sides := New[trade_side, string](struct {
		Buy  trade_side
		Sell trade_side
	}{
		Buy:  "buy",
		Sell: "sell",
	})

	if got := trade_sides.Get().Buy; got != "buy" {
		t.Fatalf("Get().Buy = %q, want %q", got, "buy")
	}

	values := trade_sides.Values()
	if len(values) != 2 {
		t.Fatalf("len(Values()) = %d, want 2", len(values))
	}
	if values[0] != "buy" || values[1] != "sell" {
		t.Fatalf("Values() = %#v, want [buy sell]", values)
	}

	native_values := trade_sides.NativeValues()
	if len(native_values) != 2 {
		t.Fatalf("len(NativeValues()) = %d, want 2", len(native_values))
	}
	if native_values[0] != "buy" || native_values[1] != "sell" {
		t.Fatalf("NativeValues() = %#v, want [buy sell]", native_values)
	}
}

func TestValues_ReturnsCopy(t *testing.T) {
	trade_sides := New[trade_side, string](struct {
		Buy  trade_side
		Sell trade_side
	}{
		Buy:  "buy",
		Sell: "sell",
	})

	values := trade_sides.Values()
	values[0] = "broken"

	values = trade_sides.Values()
	if values[0] != "buy" {
		t.Fatalf("Values() leaked mutation, got %q", values[0])
	}

	native_values := trade_sides.NativeValues()
	native_values[0] = "broken"

	native_values = trade_sides.NativeValues()
	if native_values[0] != "buy" {
		t.Fatalf("NativeValues() leaked mutation, got %q", native_values[0])
	}
}

func TestNew_PanicsOnNonStruct(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = New[trade_side, string]("buy")
}

func TestNew_PanicsOnWrongFieldType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = New[trade_side, string](struct {
		Buy int
	}{
		Buy: 1,
	})
}

func TestNew_PanicsOnEmptyEnum(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = New[trade_side, string](struct{}{})
}
