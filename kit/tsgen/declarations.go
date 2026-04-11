package tsgen

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/vormadev/vorma/kit/jsonutil"
)

/////////////////////////////////////////////////////////////////////
/////// Core
/////////////////////////////////////////////////////////////////////

// TSDrafter is a builder for producing TypeScript const, type,
// and enum declarations. It implements fmt.Stringer.
type TSDrafter struct {
	entries []string
}

/////////////////////////////////////////////////////////////////////
/////// Const methods
/////////////////////////////////////////////////////////////////////

func (d *TSDrafter) ExportConst(name string, val any) *TSDrafter {
	return d.add_const("export ", name, val)
}

func (d *TSDrafter) Const(name string, val any) *TSDrafter {
	return d.add_const("", name, val)
}

func (d *TSDrafter) add_const(prefix, name string, val any) *TSDrafter {
	d.entries = append(d.entries,
		fmt.Sprintf("%sconst %s = %s;", prefix, name, must_serialize(val)),
	)
	return d
}

/////////////////////////////////////////////////////////////////////
/////// Type methods
/////////////////////////////////////////////////////////////////////

func (d *TSDrafter) ExportType(name string, val string) *TSDrafter {
	return d.add_type("export ", name, val)
}

func (d *TSDrafter) Type(name string, val string) *TSDrafter {
	return d.add_type("", name, val)
}

func (d *TSDrafter) add_type(prefix, name, val string) *TSDrafter {
	d.entries = append(d.entries,
		fmt.Sprintf("%stype %s = %s;", prefix, name, val),
	)
	return d
}

/////////////////////////////////////////////////////////////////////
/////// Enum methods
/////////////////////////////////////////////////////////////////////

func (d *TSDrafter) ExportEnum(
	const_name, type_name string,
	val any,
) *TSDrafter {
	return d.add_enum("export ", const_name, type_name, val)
}

func (d *TSDrafter) Enum(
	const_name, type_name string,
	val any,
) *TSDrafter {
	return d.add_enum("", const_name, type_name, val)
}

func (d *TSDrafter) add_enum(
	prefix, const_name, type_name string,
	val any,
) *TSDrafter {
	d.entries = append(
		d.entries,
		fmt.Sprintf(
			"%sconst %s = %s;",
			prefix,
			const_name,
			must_serialize(val),
		),
		fmt.Sprintf(
			"%stype %s = (typeof %s)[keyof typeof %s];",
			prefix, type_name, const_name, const_name,
		),
	)
	return d
}

/////////////////////////////////////////////////////////////////////
/////// Raw method
/////////////////////////////////////////////////////////////////////

func (d *TSDrafter) Raw(content string) *TSDrafter {
	d.entries = append(d.entries, content)
	return d
}

/////////////////////////////////////////////////////////////////////
/////// String method (implements fmt.Stringer)
/////////////////////////////////////////////////////////////////////

func (d *TSDrafter) String() string {
	var sb strings.Builder
	for i, entry := range d.entries {
		sb.WriteString(entry)
		if i != len(d.entries)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

/////////////////////////////////////////////////////////////////////
/////// Union helpers
/////////////////////////////////////////////////////////////////////

// StringLiteralUnion produces a TypeScript string literal union type,
// e.g. "active" | "inactive" | "pending".
func StringLiteralUnion(values ...string) string {
	if len(values) == 0 {
		return ""
	}
	quoted := make([]string, len(values))
	for i, s := range values {
		quoted[i] = fmt.Sprintf(`"%s"`, s)
	}
	return strings.Join(quoted, " | ")
}

// Union produces a TypeScript union type,
// e.g. SuccessResponse | ErrorResponse.
func Union(types ...string) string {
	if len(types) == 0 {
		return ""
	}
	return strings.Join(types, " | ")
}

/////////////////////////////////////////////////////////////////////
/////// Utils
/////////////////////////////////////////////////////////////////////

func must_serialize(v any) string {
	json, err := jsonutil.SerializePretty(v)
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
