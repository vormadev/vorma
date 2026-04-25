package schema_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type outer_with_inner struct {
	Name  string
	Inner rec_inner
}

type rec_inner struct {
	X string
}

func (rec_inner) Schema() schema.Schema {
	return schema.Object{
		"X": schema.String{MustNotBeZero: true},
	}
}

func (outer_with_inner) Schema() schema.Schema {
	return schema.Object{
		"Name": schema.String{MustNotBeZero: true},
	}
}

type rec_slice_item struct {
	ID int
}

func (rec_slice_item) Schema() schema.Schema {
	return schema.Object{
		"ID": schema.Int{
			ValidateFunc: func(v int) error {
				if v <= 0 {
					return errors.New("ID must be positive")
				}
				return nil
			},
		},
	}
}

type slice_of_schematic struct {
	Items []rec_slice_item
}

func (slice_of_schematic) Schema() schema.Schema {
	return schema.Object{"Items": schema.Slice{}}
}

type rec_employee struct {
	ID   int
	Name string
}

func (e *rec_employee) Schema() schema.Schema {
	return schema.Object{
		"ID": schema.Int{
			ValidateFunc: func(v int) error {
				if v <= 0 {
					return errors.New("employee ID must be positive")
				}
				return nil
			},
		},
		"Name": schema.String{MustNotBeZero: true},
	}
}

type rec_company struct {
	Name      string
	Employees map[string]*rec_employee
}

func (rec_company) Schema() schema.Schema {
	return schema.Object{
		"Name": schema.String{MustNotBeZero: true},
	}
}

type rec_parent_with_config struct {
	Name   string
	Config *rec_config
}

type rec_config struct {
	Mode string
}

func (rec_config) Schema() schema.Schema {
	return schema.Object{
		"Mode": schema.String{MustNotBeZero: true},
	}
}

var active_rec_parent_schema schema.Schema

func (rec_parent_with_config) Schema() schema.Schema {
	return active_rec_parent_schema
}

type rec_node struct {
	Value    string
	Children []*rec_node
}

func (rec_node) Schema() schema.Schema {
	return schema.Object{
		"Value": schema.String{},
	}
}

type rec_user struct {
	Username string
	Profile  rec_profile
}

func (rec_user) Schema() schema.Schema {
	return schema.Object{
		"Username": schema.String{MustNotBeZero: true, MinLen: 3},
	}
}

type rec_profile struct {
	Email  string
	Status *rec_status
}

func (rec_profile) Schema() schema.Schema {
	return schema.Object{
		"Email": schema.String{
			MustBeEmail: false,
			ValidateFunc: func(v string) error {
				if v == "" {
					return nil
				}
				if !strings.Contains(v, "@") {
					return errors.New("must be a valid email address")
				}
				return nil
			},
		},
	}
}

type rec_status struct {
	Active string
}

func (s *rec_status) Schema() schema.Schema {
	return schema.Object{
		"Active": schema.String{
			MustNotBeZero: true,
			MustBeIn:      []string{"true", "false"},
		},
	}
}

type rec_leaf_root struct {
	Inner rec_leaf_inner
}

type rec_leaf_inner struct {
	Deep rec_leaf_deep
}

type rec_leaf_deep struct {
	Value string
}

type rec_any_holder struct {
	V any
}

func (rec_leaf_deep) Schema() schema.Schema {
	return schema.Object{
		"Value": schema.String{TrimSpace: true, ToLower: true},
	}
}

func (rec_leaf_root) Schema() schema.Schema {
	return schema.Object{}
}

func (rec_leaf_inner) Schema() schema.Schema {
	return schema.Object{}
}

func (rec_any_holder) Schema() schema.Schema {
	return schema.Object{}
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestRecursion_StructFields(t *testing.T) {
	_, err := schema.Enforce("o", outer_with_inner{
		Name:  "ok",
		Inner: rec_inner{X: ""},
	})
	if err == nil {
		t.Fatalf("expected error for Inner.X")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected empty-string error, got %q", err.Error())
	}
}

func TestRecursion_SliceOfSchematics(t *testing.T) {
	h := slice_of_schematic{
		Items: []rec_slice_item{{ID: 1}, {ID: 0}, {ID: 5}},
	}
	_, err := schema.Enforce("s", h)
	if err == nil {
		t.Fatalf("expected error for ID==0 element")
	}
	if !strings.Contains(err.Error(), "Items[1]") {
		t.Fatalf("expected Items[1] in error, got %q", err.Error())
	}
}

func TestRecursion_MapOfPointersToSchematics(t *testing.T) {
	c := rec_company{
		Name: "Acme",
		Employees: map[string]*rec_employee{
			"e1": {ID: 0, Name: ""},
		},
	}
	_, err := schema.Enforce("c", &c)
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "employee ID must be positive") {
		t.Errorf("missing ID error: %q", msg)
	}
	if !strings.Contains(msg, "must not be empty") {
		t.Errorf("missing Name error: %q", msg)
	}
}

func TestRecursion_NilPointerInNestedStructField(t *testing.T) {
	active_rec_parent_schema = schema.Object{
		"Name":   schema.String{MustNotBeZero: true},
		"Config": schema.Any{},
	}
	defer func() { active_rec_parent_schema = nil }()
	_, err := schema.Enforce("p", rec_parent_with_config{
		Name:   "Acme",
		Config: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	active_rec_parent_schema = schema.Object{
		"Name":   schema.String{MustNotBeZero: true},
		"Config": schema.Any{MustNotBeNil: true},
	}
	_, err = schema.Enforce("p", rec_parent_with_config{
		Name:   "Acme",
		Config: nil,
	})
	if err == nil {
		t.Fatalf("expected error for nil Config")
	}
	if !strings.Contains(err.Error(), "Config must not be nil") {
		t.Fatalf("expected Config nil error, got %q", err.Error())
	}
}

func TestRecursion_SelfReferentialTree(t *testing.T) {
	leaf1 := &rec_node{Value: "leaf1"}
	leaf2 := &rec_node{Value: "leaf2"}
	root := &rec_node{
		Value:    "root",
		Children: []*rec_node{leaf1, leaf2},
	}
	_, err := schema.Enforce("n", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecursion_ActualSelfReferentialNodeCycleDoesNotLoop(t *testing.T) {
	root := &rec_node{Value: "root"}
	root.Children = []*rec_node{root}

	_, err := schema.Enforce("n", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecursion_SelfReferentialTree_WithNilChild(t *testing.T) {
	root := &rec_node{
		Value: "root",
		Children: []*rec_node{
			{Value: "a"},
			nil,
		},
	}
	_, err := schema.Enforce("n", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecursion_SelfReferentialSliceInInterfaceDoesNotLoop(t *testing.T) {
	var items []any
	items = []any{items}

	_, err := schema.Enforce("h", &rec_any_holder{V: items})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecursion_SelfReferentialTree_EmptyValueAllowed(t *testing.T) {
	root := &rec_node{
		Value: "",
		Children: []*rec_node{
			{Value: "a"},
			{Value: ""},
		},
	}
	_, err := schema.Enforce("n", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecursion_NestedSchemaComposition(t *testing.T) {
	_, err := schema.Enforce("u", rec_user{
		Username: "ab",
		Profile:  rec_profile{Email: "bad"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "minimum length") {
		t.Errorf("missing Username error: %q", msg)
	}
	if !strings.Contains(msg, "valid email") {
		t.Errorf("missing Profile.Email error: %q", msg)
	}
}

func TestRecursion_TripleNested_UserProfileStatus(t *testing.T) {
	u := rec_user{
		Username: "alice",
		Profile:  rec_profile{Status: &rec_status{Active: "maybe"}},
	}
	_, err := schema.Enforce("u", u)
	if err == nil {
		t.Fatalf("expected error for bad Status")
	}

	u = rec_user{
		Username: "alice",
		Profile: rec_profile{
			Email:  "a@b.com",
			Status: &rec_status{Active: "true"},
		},
	}
	_, err = schema.Enforce("u", u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecursion_LeafMutationVisibleThroughRootPointer(t *testing.T) {
	r := &rec_leaf_root{
		Inner: rec_leaf_inner{
			Deep: rec_leaf_deep{Value: "  HELLO  "},
		},
	}
	_, err := schema.Enforce("r", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Inner.Deep.Value != "hello" {
		t.Fatalf("expected leaf mutation visible through root; got %q", r.Inner.Deep.Value)
	}
}

func TestRecursion_SelfReferentialMapInInterfaceDoesNotLoop(t *testing.T) {
	m := map[string]any{}
	m["self"] = m

	_, err := schema.Enforce("h", &rec_any_holder{V: m})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
