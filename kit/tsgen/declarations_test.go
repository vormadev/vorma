package tsgen

import (
	"testing"

	"github.com/vormadev/vorma/kit/enum"
)

type trade_side string

func TestEnum(t *testing.T) {
	trade_sides := enum.New[trade_side, string](struct {
		Buy  trade_side
		Sell trade_side
	}{
		Buy:  "buy",
		Sell: "sell",
	})

	got := (&TSDrafter{}).
		ExportEnum("TradeSides", "TradeSide", trade_sides).
		String()

	want := `export const TradeSides = {
  "Buy": "buy",
  "Sell": "sell"
} as const;

export type TradeSide = (typeof TradeSides)[keyof typeof TradeSides];`

	if norm(got) != norm(want) {
		t.Fatalf("Enum() mismatch\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}
