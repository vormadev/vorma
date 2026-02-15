package timer

import "testing"

func TestNilTimerMethodsDoNotPanic(t *testing.T) {
	var nilTimer *Timer

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("expected nil timer methods to be safe, got panic: %v", recovered)
		}
	}()

	nilTimer.Checkpoint("nil-checkpoint")
	nilTimer.Reset()
}

func TestConditionalFalseIsNoOp(t *testing.T) {
	timerWhenDisabled := Conditional(false)
	timerWhenDisabled.Checkpoint("disabled")
	timerWhenDisabled.Reset()
}
