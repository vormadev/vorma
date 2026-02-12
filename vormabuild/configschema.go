package vormabuild

import "github.com/vormadev/vorma/lab/jsonschema"

var VormaSchema = jsonschema.OptionalObject(jsonschema.Def{
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
		IncludeDefaults:            IncludeDefaultsSchema,
		MainBuildEntry:             MainBuildEntrySchema,
		UIVariant:                  UIVariantSchema,
		HTMLTemplateLocation:       HTMLTemplateLocationSchema,
		ClientEntry:                ClientEntrySchema,
		ClientRouteDefsFile:        ClientRouteDefsFileSchema,
		TSGenOutDir:                TSGenOutDirSchema,
		BuildtimePublicURLFuncName: BuildtimePublicURLFuncNameSchema,
	},
})

var IncludeDefaultsSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true (default), Vorma injects default watch patterns for routes, templates, and Go files.`,
	Default:     true,
})

var MainBuildEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to the Vorma build command entry point.`,
	Examples:    []string{"backend/cmd/build", "cmd/build"},
})

var UIVariantSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `The UI framework to use for client-side rendering.`,
	Enum:        []string{"react", "preact", "solid"},
})

var HTMLTemplateLocationSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your HTML template file, relative to the private static directory.`,
	Examples:    []string{"entry.go.html"},
})

var ClientEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your client-side entry file.`,
	Examples:    []string{"frontend/src/vorma.entry.tsx"},
})

var ClientRouteDefsFileSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your client route definitions file.`,
	Examples:    []string{"frontend/src/vorma.routes.ts"},
})

var TSGenOutDirSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Directory where Vorma will generate TypeScript types and configuration.`,
	Examples:    []string{"frontend/src/vorma.gen"},
})

var BuildtimePublicURLFuncNameSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Name of the global function injected by the Vite plugin for resolving public asset URLs at build time.`,
	Default:     "waveBuildtimeURL",
	Examples:    []string{"waveBuildtimeURL", "getAssetURL"},
})
