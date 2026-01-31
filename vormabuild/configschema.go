package vormabuild

import "github.com/vormadev/vorma/lab/jsonschema"

var Vorma_Schema = jsonschema.OptionalObject(jsonschema.Def{
	Description:      "Vorma framework configuration.",
	RequiredChildren: []string{"UIVariant", "HTMLTemplateLocation", "ClientEntry", "ClientRouteDefsFile", "TSGenOutDir", "MainBuildEntry"},
	Properties: struct {
		IncludeDefaults            jsonschema.Entry
		MainBuildEntry             jsonschema.Entry
		UIVariant                  jsonschema.Entry
		HTMLTemplateLocation       jsonschema.Entry
		ClientEntry                jsonschema.Entry
		ClientRouteDefsFile        jsonschema.Entry
		TSGenOutDir                jsonschema.Entry
		BuildtimePublicURLFuncName jsonschema.Entry
	}{
		IncludeDefaults:            IncludeDefaults_Schema,
		MainBuildEntry:             MainBuildEntry_Schema,
		UIVariant:                  UIVariant_Schema,
		HTMLTemplateLocation:       HTMLTemplateLocation_Schema,
		ClientEntry:                ClientEntry_Schema,
		ClientRouteDefsFile:        ClientRouteDefsFile_Schema,
		TSGenOutDir:                TSGenOutDir_Schema,
		BuildtimePublicURLFuncName: BuildtimePublicURLFuncName_Schema,
	},
})

var IncludeDefaults_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true (default), Vorma injects default watch patterns for routes, templates, and Go files.`,
	Default:     true,
})

var MainBuildEntry_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to the Vorma build command entry point.`,
	Examples:    []string{"backend/cmd/build", "cmd/build"},
})

var UIVariant_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `The UI framework to use for client-side rendering.`,
	Enum:        []string{"react", "preact", "solid"},
})

var HTMLTemplateLocation_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your HTML template file, relative to the private static directory.`,
	Examples:    []string{"entry.go.html"},
})

var ClientEntry_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your client-side entry file.`,
	Examples:    []string{"frontend/src/vorma.entry.tsx"},
})

var ClientRouteDefsFile_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your client route definitions file.`,
	Examples:    []string{"frontend/src/vorma.routes.ts"},
})

var TSGenOutDir_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Directory where Vorma will generate TypeScript types and configuration.`,
	Examples:    []string{"frontend/src/vorma.gen"},
})

var BuildtimePublicURLFuncName_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Name of the global function injected by the Vite plugin for resolving public asset URLs at build time.`,
	Default:     "waveBuildtimeURL",
	Examples:    []string{"waveBuildtimeURL", "getAssetURL"},
})
