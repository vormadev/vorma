package tsgen

import (
	"strings"
	"testing"
	"time"
)

/////////////////////////////////////////////////////////////////////
/////// FILE-SCOPE TEST STRUCTS
/////////////////////////////////////////////////////////////////////

type WalkerRawType struct{}

func (w WalkerRawType) TSType() string { return "{ custom: boolean }" }

type WalkerTSTyperStruct struct {
	X string `json:"x"`
	Y int    `json:"y"`
}

func (w WalkerTSTyperStruct) TSType() map[string]string {
	return map[string]string{"X": "custom_string"}
}

type WalkerTSTyperAdditive struct {
	A string `json:"a"`
}

func (w WalkerTSTyperAdditive) TSType() map[string]string {
	return map[string]string{"extra": "boolean"}
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func find_entry(entries map[string]*walk_entry, name string) *walk_entry {
	for _, e := range entries {
		if e.requested_name == name {
			return e
		}
	}
	return nil
}

func find_field(fields []field_node, name string) *field_node {
	for i := range fields {
		if fields[i].name == name {
			return &fields[i]
		}
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// TESTS
/////////////////////////////////////////////////////////////////////

func TestWalk(t *testing.T) {
	t.Run("NilInstance", func(t *testing.T) {
		entries, root_id := walk_type(nil, "Ignored")
		if entries != nil {
			t.Error("expected nil entries for nil instance")
		}
		if root_id != "" {
			t.Error("expected empty root ID")
		}
	})

	t.Run("RootEntry", func(t *testing.T) {
		type S struct {
			X int `json:"x"`
		}
		entries, root_id := walk_type(S{}, "MyRoot")
		if root_id == "" {
			t.Fatal("expected non-empty root ID")
		}
		e, ok := entries[root_id]
		if !ok {
			t.Fatal("root ID not found in entries")
		}
		if !e.is_root {
			t.Error("root entry should have is_root=true")
		}
		if e.requested_name != "MyRoot" {
			t.Errorf("requested_name=%q, want 'MyRoot'", e.requested_name)
		}
	})

	t.Run("PrimitiveFieldKinds", func(t *testing.T) {
		type S struct {
			B   bool    `json:"b"`
			I   int     `json:"i"`
			F   float64 `json:"f"`
			Str string  `json:"s"`
			A   any     `json:"a"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		if e == nil {
			t.Fatal("root entry not found")
		}
		if e.node.kind != kind_object {
			t.Fatalf("root should be kind_object, got %d", e.node.kind)
		}
		checks := []struct {
			name string
			want node_kind
		}{
			{"b", kind_bool},
			{"i", kind_number},
			{"f", kind_number},
			{"s", kind_string},
			{"a", kind_unknown},
		}
		for _, c := range checks {
			f := find_field(e.node.fields, c.name)
			if f == nil {
				t.Errorf("field %s not found", c.name)
				continue
			}
			if f.node.kind != c.want {
				t.Errorf(
					"field %s: got kind %d, want %d",
					c.name,
					f.node.kind,
					c.want,
				)
			}
		}
	})

	t.Run("NamedStructFieldProducesRef", func(t *testing.T) {
		type Inner struct {
			X string `json:"x"`
		}
		type Outer struct {
			I Inner `json:"i"`
		}
		entries, _ := walk_type(Outer{}, "Outer")
		outer := find_entry(entries, "Outer")
		if outer == nil {
			t.Fatal("Outer not found")
		}
		f := find_field(outer.node.fields, "i")
		if f == nil {
			t.Fatal("field 'i' not found")
		}
		if f.node.kind != kind_ref {
			t.Errorf(
				"expected kind_ref for named struct field, got %d",
				f.node.kind,
			)
		}
		if f.node.ref_id == "" {
			t.Error("ref_id should be non-empty")
		}
		inner := find_entry(entries, "Inner")
		if inner == nil {
			t.Fatal("Inner entry not found")
		}
		if !inner.is_referenced {
			t.Error("Inner should be marked as referenced")
		}
	})

	t.Run("PointerToNamedStructProducesRef", func(t *testing.T) {
		type Inner struct {
			X string `json:"x"`
		}
		type Outer struct {
			I *Inner `json:"i"`
		}
		entries, _ := walk_type(Outer{}, "Outer")
		outer := find_entry(entries, "Outer")
		f := find_field(outer.node.fields, "i")
		if f == nil {
			t.Fatal("field 'i' not found")
		}
		if f.node.kind != kind_ref {
			t.Errorf(
				"pointer to named struct should produce kind_ref, got %d",
				f.node.kind,
			)
		}
		if !f.optional {
			t.Error("pointer field should be optional")
		}
	})

	t.Run("RefIDIsOpaque", func(t *testing.T) {
		type Inner struct {
			X string `json:"x"`
		}
		type Outer struct {
			I Inner `json:"i"`
		}
		entries, _ := walk_type(Outer{}, "Outer")
		outer := find_entry(entries, "Outer")
		f := find_field(outer.node.fields, "i")
		if f == nil {
			t.Fatal("field 'i' not found")
		}
		if f.node.ref_id == "" {
			t.Error("ref_id should be non-empty")
		}
		if strings.Contains(f.node.ref_id, "Inner") {
			t.Error("ref_id should not leak type name")
		}
	})

	t.Run("EmbeddedStructFlags", func(t *testing.T) {
		type Base struct {
			Name string `json:"name"`
		}
		type Host struct {
			ID string `json:"id"`
			Base
		}
		entries, _ := walk_type(Host{}, "Host")
		base := find_entry(entries, "Base")
		if base == nil {
			t.Fatal("Base entry not found")
		}
		if !base.used_as_embedded {
			t.Error("Base should have used_as_embedded=true")
		}
		if base.is_referenced {
			t.Error("untagged embedded Base should not be is_referenced")
		}
	})

	t.Run("TaggedEmbeddedIsReferenced", func(t *testing.T) {
		type Base struct {
			Name string `json:"name"`
		}
		type Host struct {
			Base `json:"base"`
		}
		entries, _ := walk_type(Host{}, "Host")
		base := find_entry(entries, "Base")
		if base == nil {
			t.Fatal("Base entry not found")
		}
		if !base.used_as_embedded {
			t.Error("should have used_as_embedded=true")
		}
		if !base.is_referenced {
			t.Error("tagged embedded should have is_referenced=true")
		}
	})

	t.Run("JsonDashOmitted", func(t *testing.T) {
		type S struct {
			Keep   string `json:"keep"`
			Ignore string `json:"-"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		if e == nil {
			t.Fatal("entry not found")
		}
		if find_field(e.node.fields, "Ignore") != nil {
			t.Error("json:\"-\" field should be omitted from fields")
		}
		if find_field(e.node.fields, "keep") == nil {
			t.Error("kept field should be present")
		}
	})

	t.Run("JsonDashCommaOmitted", func(t *testing.T) {
		type S struct {
			Keep   string `json:"keep"`
			Ignore any    `json:"-,"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		if find_field(e.node.fields, "Ignore") != nil {
			t.Error("json:\"-,\" field should be omitted")
		}
	})

	t.Run("TSTyperRawShortCircuits", func(t *testing.T) {
		entries, root_id := walk_type(WalkerRawType{}, "RawType")
		if root_id == "" {
			t.Fatal("expected non-empty root ID")
		}
		e := find_entry(entries, "RawType")
		if e == nil {
			t.Fatal("entry not found")
		}
		if e.node.kind != kind_raw {
			t.Errorf("expected kind_raw, got %d", e.node.kind)
		}
		if e.node.raw_ts != "{ custom: boolean }" {
			t.Errorf("wrong raw_ts: %s", e.node.raw_ts)
		}
		if len(entries) != 1 {
			t.Errorf(
				"TSTyperRaw should short-circuit; expected 1 entry, got %d",
				len(entries),
			)
		}
	})

	t.Run("TSTyperRawViaPointer", func(t *testing.T) {
		entries, root_id := walk_type(&WalkerRawType{}, "RawPtr")
		if root_id == "" {
			t.Fatal("expected non-empty root ID")
		}
		e := find_entry(entries, "RawPtr")
		if e == nil {
			t.Fatal("entry not found")
		}
		if e.node.kind != kind_raw {
			t.Errorf(
				"pointer to TSTyperRaw should still short-circuit, got kind %d",
				e.node.kind,
			)
		}
	})

	t.Run("PointerTransparency", func(t *testing.T) {
		type S struct {
			P *int `json:"p"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		f := find_field(e.node.fields, "p")
		if f == nil {
			t.Fatal("field not found")
		}
		if f.node.kind != kind_number {
			t.Errorf(
				"pointer to int should resolve to kind_number, got %d",
				f.node.kind,
			)
		}
		if !f.optional {
			t.Error("pointer field should be optional")
		}
	})

	t.Run("SliceAndArrayNodes", func(t *testing.T) {
		type S struct {
			Sl []int     `json:"sl"`
			Ar [3]string `json:"ar"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")

		sl := find_field(e.node.fields, "sl")
		if sl == nil {
			t.Fatal("field sl not found")
		}
		if sl.node.kind != kind_array {
			t.Errorf("expected kind_array for slice, got %d", sl.node.kind)
		}
		if sl.node.elem == nil || sl.node.elem.kind != kind_number {
			t.Error("slice element should be kind_number")
		}

		ar := find_field(e.node.fields, "ar")
		if ar == nil {
			t.Fatal("field ar not found")
		}
		if ar.node.kind != kind_array {
			t.Errorf("expected kind_array for array, got %d", ar.node.kind)
		}
		if ar.node.elem == nil || ar.node.elem.kind != kind_string {
			t.Error("array element should be kind_string")
		}
	})

	t.Run("MapNodes", func(t *testing.T) {
		type S struct {
			M map[string]int `json:"m"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		m := find_field(e.node.fields, "m")
		if m == nil {
			t.Fatal("field m not found")
		}
		if m.node.kind != kind_map {
			t.Fatalf("expected kind_map, got %d", m.node.kind)
		}
		if m.node.key_type == nil || m.node.key_type.kind != kind_string {
			t.Error("map key should be kind_string")
		}
		if m.node.val_type == nil || m.node.val_type.kind != kind_number {
			t.Error("map value should be kind_number")
		}
	})

	t.Run("TimeFieldIsString", func(t *testing.T) {
		type S struct {
			T time.Time `json:"t"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		f := find_field(e.node.fields, "t")
		if f == nil {
			t.Fatal("field not found")
		}
		if f.node.kind != kind_string {
			t.Errorf("time.Time should be kind_string, got %d", f.node.kind)
		}
	})

	t.Run("DurationFieldIsNumber", func(t *testing.T) {
		type S struct {
			D time.Duration `json:"d"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		f := find_field(e.node.fields, "d")
		if f == nil {
			t.Fatal("field not found")
		}
		if f.node.kind != kind_number {
			t.Errorf("time.Duration should be kind_number, got %d", f.node.kind)
		}
	})

	t.Run("ByteSliceIsString", func(t *testing.T) {
		type S struct {
			Data []byte `json:"data"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		f := find_field(e.node.fields, "data")
		if f == nil {
			t.Fatal("field not found")
		}
		if f.node.kind != kind_string {
			t.Errorf("[]byte should be kind_string, got %d", f.node.kind)
		}
	})

	t.Run("TSTypeTagProducesRawNode", func(t *testing.T) {
		type S struct {
			X string `json:"x" ts_type:"CustomType"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		f := find_field(e.node.fields, "x")
		if f == nil {
			t.Fatal("field not found")
		}
		if f.node.kind != kind_raw {
			t.Errorf("ts_type tag should produce kind_raw, got %d", f.node.kind)
		}
		if f.node.raw_ts != "CustomType" {
			t.Errorf("raw_ts=%q, want 'CustomType'", f.node.raw_ts)
		}
	})

	t.Run("TSTyperMethodProducesRawNode", func(t *testing.T) {
		entries, _ := walk_type(WalkerTSTyperStruct{}, "T")
		e := find_entry(entries, "T")
		if e == nil {
			t.Fatal("entry not found")
		}

		// X is overridden by TSTyper.
		fx := find_field(e.node.fields, "x")
		if fx == nil {
			t.Fatal("field x not found")
		}
		if fx.node.kind != kind_raw {
			t.Errorf(
				"TSTyper override should produce kind_raw, got %d",
				fx.node.kind,
			)
		}
		if fx.node.raw_ts != "custom_string" {
			t.Errorf("raw_ts=%q, want 'custom_string'", fx.node.raw_ts)
		}

		// Y is not overridden; should retain its reflected kind.
		fy := find_field(e.node.fields, "y")
		if fy == nil {
			t.Fatal("field y not found")
		}
		if fy.node.kind != kind_number {
			t.Errorf(
				"non-overridden field should be kind_number, got %d",
				fy.node.kind,
			)
		}
	})

	t.Run("TSTyperAdditiveFields", func(t *testing.T) {
		entries, _ := walk_type(WalkerTSTyperAdditive{}, "T")
		e := find_entry(entries, "T")
		if e == nil {
			t.Fatal("entry not found")
		}

		// "a" is a real Go field.
		fa := find_field(e.node.fields, "a")
		if fa == nil {
			t.Fatal("field a not found")
		}
		if fa.node.kind != kind_string {
			t.Errorf("field a should be kind_string, got %d", fa.node.kind)
		}

		// "extra" is additive from TSTyper (no corresponding Go field).
		fextra := find_field(e.node.fields, "extra")
		if fextra == nil {
			t.Fatal("additive field 'extra' not found")
		}
		if fextra.node.kind != kind_raw {
			t.Errorf(
				"additive field should be kind_raw, got %d",
				fextra.node.kind,
			)
		}
		if fextra.node.raw_ts != "boolean" {
			t.Errorf("raw_ts=%q, want 'boolean'", fextra.node.raw_ts)
		}
		if fextra.optional {
			t.Error("additive fields should be required (not optional)")
		}
	})

	t.Run("OptionalFieldDetection", func(t *testing.T) {
		type S struct {
			Ptr       *string `json:"ptr"`
			OmitEmpty string  `json:"oe,omitempty"`
			OmitZero  int     `json:"oz,omitzero"`
			Required  string  `json:"req"`
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		if e == nil {
			t.Fatal("entry not found")
		}
		cases := []struct {
			name     string
			want_opt bool
		}{
			{"ptr", true},
			{"oe", true},
			{"oz", true},
			{"req", false},
		}
		for _, c := range cases {
			f := find_field(e.node.fields, c.name)
			if f == nil {
				t.Errorf("field %s not found", c.name)
				continue
			}
			if f.optional != c.want_opt {
				t.Errorf(
					"field %s: optional=%v, want %v",
					c.name,
					f.optional,
					c.want_opt,
				)
			}
		}
	})

	t.Run("EmptyStructProducesEmptyObject", func(t *testing.T) {
		type Empty struct{}
		entries, _ := walk_type(Empty{}, "Empty")
		e := find_entry(entries, "Empty")
		if e == nil {
			t.Fatal("entry not found")
		}
		if e.node.kind != kind_object {
			t.Errorf("expected kind_object, got %d", e.node.kind)
		}
		if len(e.node.fields) != 0 {
			t.Errorf("expected 0 fields, got %d", len(e.node.fields))
		}
	})

	t.Run("UnexportedFieldsOmitted", func(t *testing.T) {
		type S struct {
			Public  string `json:"public"`
			private string //nolint:unused
		}
		entries, _ := walk_type(S{}, "S")
		e := find_entry(entries, "S")
		if len(e.node.fields) != 1 {
			t.Errorf(
				"expected 1 field (unexported omitted), got %d",
				len(e.node.fields),
			)
		}
		if find_field(e.node.fields, "public") == nil {
			t.Error("public field should be present")
		}
	})

	t.Run("EmbeddedPtrFieldsOptional", func(t *testing.T) {
		type Base struct {
			Name string `json:"name"`
		}
		type Host struct {
			*Base
		}
		entries, _ := walk_type(Host{}, "Host")
		host := find_entry(entries, "Host")
		if host == nil {
			t.Fatal("host entry not found")
		}
		f := find_field(host.node.fields, "name")
		if f == nil {
			t.Fatal("flattened field 'name' not found")
		}
		if !f.optional {
			t.Error("field from pointer-embedded struct should be optional")
		}
	})

	t.Run("PointerInstanceDereferenced", func(t *testing.T) {
		type S struct {
			X int `json:"x"`
		}
		entries, root_id := walk_type(&S{}, "S")
		if root_id == "" {
			t.Fatal("expected non-empty root ID")
		}
		e := find_entry(entries, "S")
		if e == nil {
			t.Fatal("entry not found after pointer dereference")
		}
		if e.node.kind != kind_object {
			t.Errorf(
				"dereferenced pointer should produce kind_object, got %d",
				e.node.kind,
			)
		}
	})
}
