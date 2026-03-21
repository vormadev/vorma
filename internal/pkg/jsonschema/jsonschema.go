// Package jsonschema provides small helpers for building JSON schema payloads
// from declarative Go definitions.
//
// The API is intentionally narrow and focused on repository needs (primarily
// Wave/Vorma config schema generation) rather than full JSON Schema feature
// coverage.
//
// NOTE:
// This package primarily exists for Wave's JSON schema generation.
// It does not, and probably will not, cover the entire JSON Schema spec.
package jsonschema

import (
	"fmt"
	"strings"

	"github.com/vormadev/vorma/lab/stringsutil"
)

const (
	TypeObject  = "object"
	TypeString  = "string"
	TypeBoolean = "boolean"
	TypeArray   = "array"
	TypeNumber  = "number"
)

// Def describes a schema field before conversion to an Entry.
type Def struct {
	Type                string
	Required            bool
	Description         string
	DescriptionOverride string
	Examples            []string
	Default             any
	MinItems            int
	MaxItems            int
	RequiredChildren    []string
	Properties          any
	AllOf               []any
	Items               Entry
	Enum                []string
}

// Entry is a JSON-serializable schema fragment.
type Entry struct {
	Schema      string   `json:"$schema,omitempty"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     any      `json:"default,omitempty"`
	Required    []string `json:"required,omitempty"`
	AllOf       []any    `json:"allOf,omitempty"`
	Properties  any      `json:"properties,omitempty"`
	Items       any      `json:"items,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	MinItems    int      `json:"minItems,omitempty"`
	MaxItems    int      `json:"maxItems,omitempty"`
}

// IfThen models a JSON schema conditional branch.
type IfThen struct {
	If   any `json:"if,omitempty"`
	Then any `json:"then,omitempty"`
}

// ToJSONSchema converts Def into an Entry.
func ToJSONSchema(sd Def) Entry {
	x := Entry{
		Type:        sd.Type,
		Description: sd.descStr(),
		Required:    sd.RequiredChildren,
		Default:     sd.Default,
		Examples:    sd.Examples,
		AllOf:       sd.AllOf,
		Properties:  sd.Properties,
		Enum:        sd.Enum,
		MinItems:    sd.MinItems,
		MaxItems:    sd.MaxItems,
	}
	if sd.Items.Type != "" {
		x.Items = sd.Items
	}
	return x
}

func (e Entry) IsZero() bool {
	return e.Schema == "" &&
		e.Type == "" &&
		e.Description == "" &&
		e.Default == nil &&
		len(e.Required) == 0 &&
		len(e.AllOf) == 0 &&
		e.Properties == nil &&
		e.Items == nil &&
		len(e.Enum) == 0 &&
		len(e.Examples) == 0 &&
		e.MinItems == 0 &&
		e.MaxItems == 0
}

func (d Def) descStr() string {
	x := stringsutil.Builder{}

	if d.DescriptionOverride != "" {
		x.Write(d.DescriptionOverride)
	} else {
		if d.Required {
			x.Write("Required")
		} else {
			x.Write("Optional")
		}
		x.Space()
		x.Write(d.Type)
		x.Write(".")
		if d.Description != "" {
			x.Space()
			x.Write(d.Description)
		}
	}

	if d.Default != nil {
		x.Return()
		x.Return()
		x.Write("Default: ")
		defaultToUse := fmt.Sprintf("%v", d.Default)
		// If the default value is a string, add quotes to make it valid JSON.
		if defaultStringValue, defaultIsString := d.Default.(string); defaultIsString {
			defaultToUse = fmt.Sprintf("%q", defaultStringValue)
		}
		x.Write(defaultToUse)
	}

	if len(d.Examples) > 0 {
		x.Return()
		x.Return()
		if len(d.Examples) == 1 {
			x.Write("Example: ")
		} else {
			x.Write("Examples: ")
		}
		x.Write(toOxfordList(d.Examples, "or"))
	}

	return x.String()
}

func toOxfordList(items []string, conjunction string) string {
	length := len(items)
	if length == 0 {
		return ""
	}
	if length == 1 {
		return fmt.Sprintf("%q", items[0])
	}
	quotedItems := make([]string, length)
	for i, item := range items {
		quotedItems[i] = fmt.Sprintf("%q", item)
	}
	if length == 2 {
		return strings.Join(quotedItems, " "+conjunction+" ")
	}
	lastItem := quotedItems[length-1]
	initialItems := quotedItems[:length-1]
	return fmt.Sprintf(
		"%s, %s %s",
		strings.Join(initialItems, ", "),
		conjunction,
		lastItem,
	)
}

// UniqueFrom builds a common uniqueness constraint description.
func UniqueFrom(strs ...string) string {
	return "Must be unique from " + toOxfordList(strs, "and")
}

// RequiredObject builds a required object schema entry.
func RequiredObject(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeObject
	return ToJSONSchema(sd)
}

// RequiredString builds a required string schema entry.
func RequiredString(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeString
	return ToJSONSchema(sd)
}

// RequiredBoolean builds a required boolean schema entry.
func RequiredBoolean(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeBoolean
	return ToJSONSchema(sd)
}

// RequiredArray builds a required array schema entry.
func RequiredArray(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeArray
	return ToJSONSchema(sd)
}

// OptionalObject builds an optional object schema entry.
func OptionalObject(sd Def) Entry {
	sd.Type = TypeObject
	sd.Required = false
	return ToJSONSchema(sd)
}

// OptionalString builds an optional string schema entry.
func OptionalString(sd Def) Entry {
	sd.Type = TypeString
	sd.Required = false
	return ToJSONSchema(sd)
}

// OptionalBoolean builds an optional boolean schema entry.
func OptionalBoolean(sd Def) Entry {
	sd.Type = TypeBoolean
	sd.Required = false
	return ToJSONSchema(sd)
}

// OptionalArray builds an optional array schema entry.
func OptionalArray(sd Def) Entry {
	sd.Type = TypeArray
	sd.Required = false
	return ToJSONSchema(sd)
}

// ObjectWithOverride builds an object schema entry with explicit description text.
func ObjectWithOverride(override string, sd Def) Entry {
	sd.Type = TypeObject
	sd.DescriptionOverride = override
	return ToJSONSchema(sd)
}

// OptionalNumber builds an optional number schema entry.
func OptionalNumber(sd Def) Entry {
	sd.Type = TypeNumber
	sd.Required = false
	return ToJSONSchema(sd)
}
