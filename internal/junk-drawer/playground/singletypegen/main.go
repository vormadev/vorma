package main

import (
	"fmt"

	"github.com/vormadev/vorma/lab/tsgen/tsgencore"
)

// Basic types
type SimpleString struct {
	Name string
}

type SimpleInt struct {
	Age int
}

// Nested types
type Person struct {
	Name string
	Age  int
}

type Family struct {
	Father Person
	Mother Person
}

// Pointer types
type NullablePerson struct {
	Name *string
	Age  *int
}

// Slice types
type Team struct {
	Members []Person
}

// Map types
type Scores struct {
	PlayerScores map[string]int
}

// Complex nested types
type ComplexStruct struct {
	Name     string
	Details  map[string]*Person
	Families []Family
	Scores   Scores
	Nullable *NullablePerson
}

// Recursive types
type Node struct {
	Value    int
	Children []*Node
}

// Multiple similar types
type AnotherPerson struct {
	Name string
	Age  int
}

type People struct {
	Persons        []Person
	AnotherPersons []AnotherPerson
}

// Type with embedded structs
type Employee struct {
	Person
	Company string
}

// Interface type (tricky case)
type Container struct {
	Value any
}

// Deeply nested types

type DeepNesting struct{ A A }
type (
	A struct{ B B }
	B struct{ C C }
	C struct{ D string }
)

type DeepNestingAnon struct {
	A struct {
		B struct {
			C struct {
				D string
			}
		}
	}
}

// Array types
type ArrayStruct struct {
	Numbers [5]int
}

// Slice of pointers
type PointerSlice struct {
	People []*Person
}

type EmbeddedStruct struct {
	*Person
	Family
	Company string
}

func prettyPrint(x tsgencore.Results) {
	fmt.Println()
	for k, v := range x.Types {
		fmt.Println("Key:", k)
		fmt.Println("Alias:", fmt.Sprintf("%v", v.ResolvedName))
		fmt.Println("OriginalName:", fmt.Sprintf("%v", v.OriginalName))
		fmt.Println("Core:", v.TSStr)
	}
	fmt.Println()
}

func main() {
	// a := tsgencore.TraverseType(EmbeddedStruct{}, "Bob")
	// prettyPrint(a)
	a := tsgencore.ProcessTypes([]*tsgencore.AdHocType{
		// same type, different name
		{TypeInstance: SimpleString{}, TSTypeName: "SimpleString"},
		{TypeInstance: struct{ Name string }{}, TSTypeName: "SimpleString2"},

		// same name, different types
		{TypeInstance: struct{ Name2 string }{}, TSTypeName: "Bob"},
		{TypeInstance: struct{ Name2 string }{}, TSTypeName: "Bob2"},
		{TypeInstance: struct{ Name4 string }{}, TSTypeName: "SimpleString"},

		{TypeInstance: SimpleInt{}, TSTypeName: "SimpleInt"},
		{TypeInstance: Person{}, TSTypeName: "Person"},
		{TypeInstance: Family{}, TSTypeName: "Family"},
		{TypeInstance: NullablePerson{}, TSTypeName: "NullablePerson"},
		{TypeInstance: Team{}, TSTypeName: "Team"},
		{TypeInstance: Scores{}, TSTypeName: "Scores"},
		{TypeInstance: ComplexStruct{}, TSTypeName: "ComplexStruct"},
		{TypeInstance: Node{}, TSTypeName: "Node"},
		{TypeInstance: People{}, TSTypeName: "People"},
		{TypeInstance: Employee{}, TSTypeName: "Employee"},
		{TypeInstance: Container{}, TSTypeName: "Container"},
		{TypeInstance: DeepNesting{}, TSTypeName: "DeepNesting"},
		{TypeInstance: DeepNestingAnon{}, TSTypeName: "DeepNestingAnon"},
		{TypeInstance: ArrayStruct{}, TSTypeName: "ArrayStruct"},
		{TypeInstance: PointerSlice{}, TSTypeName: "PointerSlice"},
		{TypeInstance: EmbeddedStruct{}, TSTypeName: "EmbeddedStruct"},
	})

	prettyPrint(a)
}
