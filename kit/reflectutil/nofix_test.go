// Code generated [NOTE: THIS IS A HACK TO PREVENT GOFIX]. DO NOT EDIT.

package reflectutil

import (
	"reflect"
	"testing"
)

func TestPublicStructFields_InvalidInlineOptions(t *testing.T) {
	type sample struct {
		Inline struct{} `json:",inline,omitempty"`
	}
	if _, err := PublicStructFields(reflect.TypeFor[sample]()); err == nil {
		t.Fatalf("expected error for invalid inline options")
	}
}
