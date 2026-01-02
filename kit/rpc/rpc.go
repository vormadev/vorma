package rpc

import (
	"strings"
	"text/template"

	"github.com/vormadev/vorma/kit/tsgen"
)

const CollectionVarName = "routes"

type RouteDef = struct {
	Key        string
	ActionType ActionType
	Input      any
	Output     any
}

type ActionType = string

const (
	ActionTypeQuery    ActionType = "query"
	ActionTypeMutation ActionType = "mutation"
)

type AdHocType = tsgen.AdHocType

type Opts struct {
	// Path, including filename, where the resulting TypeScript file will be written
	OutPath           string
	RouteDefs         []RouteDef
	AdHocTypes        []*AdHocType
	ExportRoutesArray bool
	ExtraTSCode       string
}

func GenerateTypeScript(opts Opts) error {
	var items []tsgen.CollectionItem

	for _, r := range opts.RouteDefs {
		items = append(items, tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{"key": r.Key, "actionType": r.ActionType},
			PhantomTypes: map[string]AdHocType{
				"phantomInputType":  {TypeInstance: r.Input},
				"phantomOutputType": {TypeInstance: r.Output},
			},
		})
	}

	var extraTSToUse string
	if len(opts.RouteDefs) > 0 {
		extraTSToUse = extraTSCode
	}
	if opts.ExtraTSCode != "" {
		extraTSToUse += "\n" + opts.ExtraTSCode
	}

	return tsgen.GenerateTSToFile(tsgen.Opts{
		OutPath:               opts.OutPath,
		AdHocTypes:            opts.AdHocTypes,
		Collection:            items,
		ExtraTSCode:           extraTSToUse,
		CollectionVarName:     CollectionVarName,
		ExportCollectionArray: opts.ExportRoutesArray,
	})
}

var baseOptions = BaseOptions{
	CollectionVarName:    CollectionVarName,
	DiscriminatorStr:     "key",
	CategoryPropertyName: "actionType",
}

var extraTSCode = BuildFromCategories(
	[]CategorySpecificOptions{
		{
			BaseOptions:          baseOptions,
			CategoryValue:        ActionTypeQuery,
			ItemTypeNameSingular: "QueryAPIRoute",
			ItemTypeNamePlural:   "QueryAPIRoutes",
			KeyUnionTypeName:     "QueryAPIKey",
			InputUnionTypeName:   "QueryAPIInput",
			OutputUnionTypeName:  "QueryAPIOutput",
		},
		{
			BaseOptions:          baseOptions,
			CategoryValue:        ActionTypeMutation,
			ItemTypeNameSingular: "MutationAPIRoute",
			ItemTypeNamePlural:   "MutationAPIRoutes",
			KeyUnionTypeName:     "MutationAPIKey",
			InputUnionTypeName:   "MutationAPIInput",
			OutputUnionTypeName:  "MutationAPIOutput",
		},
	},
)

type BaseOptions struct {
	CollectionVarName    string
	DiscriminatorStr     string
	CategoryPropertyName string
}

type CategorySpecificOptions struct {
	BaseOptions
	CategoryValue        string
	ItemTypeNameSingular string
	ItemTypeNamePlural   string
	KeyUnionTypeName     string
	InputUnionTypeName   string
	OutputUnionTypeName  string
	SkipInput            bool
	SkipOutput           bool
}

func BuildFromCategories(categories []CategorySpecificOptions) string {
	var extraTSBuilder strings.Builder

	for i, c := range categories {
		// START
		if err := baseTmpl.Execute(&extraTSBuilder, c); err != nil {
			panic(err)
		}
		extraTSBuilder.WriteString("\n")

		// INPUT
		if !c.SkipInput {
			if err := inputTmpl.Execute(&extraTSBuilder, c); err != nil {
				panic(err)
			}
			extraTSBuilder.WriteString("\n")
		}

		// OUTPUT
		if !c.SkipOutput {
			if err := outputTmpl.Execute(&extraTSBuilder, c); err != nil {
				panic(err)
			}
			extraTSBuilder.WriteString("\n")
		}

		if i < len(categories)-1 {
			extraTSBuilder.WriteString("\n")
		}
	}

	return extraTSBuilder.String()
}

/////////////////////////////////////////////////////////////////////
/////// MESSY TEMPLATES
/////////////////////////////////////////////////////////////////////

var (
	baseTmpl   = template.Must(template.New("extraTS_1").Parse(baseTmplStr))
	inputTmpl  = template.Must(template.New("extraTS_2").Parse(inputTmplStr))
	outputTmpl = template.Must(template.New("extraTS_3").Parse(outputTmplStr))
)

const (
	baseTmplStr = `export type {{ .ItemTypeNameSingular }} = Extract<
	(typeof {{ .CollectionVarName }})[number],
	{ {{ .CategoryPropertyName }}: "{{ .CategoryValue }}" }
>;
export type {{ .ItemTypeNamePlural }} = {
	[K in {{ .KeyUnionTypeName }}]: Extract<{{ .ItemTypeNameSingular }}, { {{ .DiscriminatorStr }}: K }>;
};
export type {{ .KeyUnionTypeName }} = {{ .ItemTypeNameSingular }}["{{ .DiscriminatorStr }}"];`

	inputTmplStr = `export type {{ .InputUnionTypeName }}<T extends {{ .KeyUnionTypeName }}> =
	Extract<{{ .ItemTypeNameSingular }}, { {{ .DiscriminatorStr }}: T }> extends { phantomInputType: infer U }
		? U
		: null | undefined;`

	outputTmplStr = `export type {{ .OutputUnionTypeName }}<T extends {{ .KeyUnionTypeName }}> =
	Extract<{{ .ItemTypeNameSingular }}, { {{ .DiscriminatorStr }}: T }> extends { phantomOutputType: infer U }
		? U
		: null | undefined;`
)
