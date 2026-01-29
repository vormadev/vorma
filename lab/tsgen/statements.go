package tsgen

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type Statements [][2]string

func (m *Statements) Raw(prefix string, value string) *Statements {
	*m = append(*m, [2]string{prefix, value})
	return m
}

func (m *Statements) Serialize(prefix string, value any) *Statements {
	*m = append(*m, [2]string{prefix, serialize(value)})
	return m
}

func (m *Statements) Enum(constName, typeName string, enumStruct any) *Statements {
	m.Serialize(fmt.Sprintf("export const %s", constName), enumStruct)
	m.Raw(fmt.Sprintf("export type %s", typeName), fmt.Sprintf("(typeof %s)[keyof typeof %s]", constName, constName))
	return m
}

func (m *Statements) BuildString() string {
	var code strings.Builder

	for _, def := range *m {
		code.WriteString(def[0])
		code.WriteString(" = ")
		code.WriteString(def[1])
		code.WriteString(";\n")
	}

	return code.String()
}

func serialize(v any) string {
	json, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		panic(err)
	}

	code := string(json)

	kind := reflect.TypeOf(v).Kind()
	if kind != reflect.String && kind != reflect.Int && kind != reflect.Bool {
		code += " as const"
	}

	return code
}

func StringUnion(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	quoted := make([]string, len(strs))
	for i, s := range strs {
		quoted[i] = fmt.Sprintf("'%s'", s)
	}
	return strings.Join(quoted, " | ")
}

func TypeUnion(typeVars []string) string {
	if len(typeVars) == 0 {
		return ""
	}
	return strings.Join(typeVars, " | ")
}
