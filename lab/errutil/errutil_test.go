package errutil

import (
	"errors"
	"testing"
)

func TestErrError_ComposesWrapperPrefixAndCause(t *testing.T) {
	wrapperErr := errors.New("wrapper")
	causeErr := errors.New("cause")

	err := Err{
		Wrapper: wrapperErr,
		Prefix:  "prefix",
		Cause:   causeErr,
	}

	if got := err.Error(); got != "wrapper: prefix: cause" {
		t.Fatalf("Error() = %q, want %q", got, "wrapper: prefix: cause")
	}
}

func TestErrError_DoesNotDuplicateCauseWhenWrapperMatchesCause(t *testing.T) {
	sameErr := errors.New("same")
	err := Err{
		Wrapper: sameErr,
		Cause:   sameErr,
	}

	if got := err.Error(); got != "same" {
		t.Fatalf("Error() = %q, want %q", got, "same")
	}
}

func TestErrUnwrap_ExposesWrapperAndCause(t *testing.T) {
	wrapperErr := errors.New("wrapper")
	causeErr := errors.New("cause")
	err := Err{
		Wrapper: wrapperErr,
		Cause:   causeErr,
	}

	unwrapped := err.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("len(Unwrap()) = %d, want %d", len(unwrapped), 2)
	}
	if unwrapped[0] != wrapperErr || unwrapped[1] != causeErr {
		t.Fatalf("unexpected unwrapped errors: %#v", unwrapped)
	}
	if !errors.Is(err, wrapperErr) {
		t.Fatal("errors.Is should match wrapper error")
	}
	if !errors.Is(err, causeErr) {
		t.Fatal("errors.Is should match cause error")
	}
}

func TestMaybe_ReturnsNilForNilInputError(t *testing.T) {
	if got := Maybe("ignored", nil); got != nil {
		t.Fatalf("Maybe() = %v, want nil", got)
	}
}

func TestMaybe_WrapsNonNilInputError(t *testing.T) {
	causeErr := errors.New("boom")

	got := Maybe("prefix", causeErr)
	if got == nil {
		t.Fatal("Maybe() returned nil for non-nil error")
	}
	if got.Error() != "prefix: boom" {
		t.Fatalf("Maybe() error = %q, want %q", got.Error(), "prefix: boom")
	}
	if !errors.Is(got, causeErr) {
		t.Fatal("Maybe() should preserve error wrapping for errors.Is")
	}
}

func TestToIsErrFunc_UsesErrorsIsSemantics(t *testing.T) {
	targetErr := errors.New("target")
	wrappedErr := Maybe("prefix", targetErr)

	isTargetErr := ToIsErrFunc(targetErr)
	if !isTargetErr(wrappedErr) {
		t.Fatal("expected predicate to match wrapped target error")
	}
	if isTargetErr(errors.New("different")) {
		t.Fatal("predicate should not match a different error")
	}
}
