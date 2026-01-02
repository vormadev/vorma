/*
NOTE:

This package primarily exists for Wave's JSON schema generation.
It does not -- and probably won't ever -- cover the entire JSON
schema spec (or anywhere near it).

Buyer beware.
*/
package jsonschema

import (
	"fmt"
	"strings"

	"github.com/vormadev/vorma/kit/stringsutil"
)

const (
	TypeObject  = "object"
	TypeString  = "string"
	TypeBoolean = "boolean"
	TypeArray   = "array"
	TypeNumber  = "number"
)

type Def struct {
	Type                string
	Required            bool
	Description         string
	DescriptionOverride string
	Examples            []string
	Default             any
	RequiredChildren    []string
	Properties          any
	AllOf               []any
	Items               Entry
	Enum                []string
}

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
}

type IfThen struct {
	If   any `json:"if,omitempty"`
	Then any `json:"then,omitempty"`
}

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
	}
	if sd.Items.Type != "" {
		x.Items = sd.Items
	}
	return x
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
		defaultToUse := d.Default
		// If the default is a string, we need to add quotes around it
		// to make it valid JSON.
		if d.Type == TypeString {
			defaultToUse = fmt.Sprintf("%q", d.Default)
		}
		x.Write(fmt.Sprintf("%v", defaultToUse))
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
	return fmt.Sprintf("%s, or %s", strings.Join(initialItems, ", "), lastItem)
}

func UniqueFrom(strs ...string) string {
	return "Must be unique from " + toOxfordList(strs, "and")
}

func RequiredObject(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeObject
	return ToJSONSchema(sd)
}
func RequiredString(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeString
	return ToJSONSchema(sd)
}
func RequiredBoolean(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeBoolean
	return ToJSONSchema(sd)
}
func RequiredArray(sd Def) Entry {
	sd.Required = true
	sd.Type = TypeArray
	return ToJSONSchema(sd)
}

func OptionalObject(sd Def) Entry {
	sd.Type = TypeObject
	sd.Required = false
	return ToJSONSchema(sd)
}
func OptionalString(sd Def) Entry {
	sd.Type = TypeString
	sd.Required = false
	return ToJSONSchema(sd)
}
func OptionalBoolean(sd Def) Entry {
	sd.Type = TypeBoolean
	sd.Required = false
	return ToJSONSchema(sd)
}
func OptionalArray(sd Def) Entry {
	sd.Type = TypeArray
	sd.Required = false
	return ToJSONSchema(sd)
}

func ObjectWithOverride(override string, sd Def) Entry {
	sd.Type = TypeObject
	sd.DescriptionOverride = override
	return ToJSONSchema(sd)
}

func OptionalNumber(sd Def) Entry {
	sd.Type = TypeNumber
	sd.Required = false
	return ToJSONSchema(sd)
}
