package vormabuild

import "github.com/vormadev/vorma/lab/jsonschema"

var VormaSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: "Vorma framework configuration.",
	RequiredChildren: []string{
		"MainBuildEntry",
		"UIVariant",
		"HTMLTemplateLocation",
		"ClientEntry",
		"ClientRouteDefinitionPatterns",
		"TSGenOutDir",
	},
	Properties: struct {
		IncludeDefaults               jsonschema.Entry
		MainBuildEntry                jsonschema.Entry
		UIVariant                     jsonschema.Entry
		HTMLTemplateLocation          jsonschema.Entry
		ClientEntry                   jsonschema.Entry
		ClientRouteDefinitionPatterns jsonschema.Entry
		ServerRouteDefinitionPatterns jsonschema.Entry
		TSGenOutDir                   jsonschema.Entry
		BuildtimePublicURLFuncName    jsonschema.Entry
		DevReloadRoutesEndpointPath   jsonschema.Entry
		DevReloadTemplateEndpointPath jsonschema.Entry
		TemplateDataKeyHeadElements   jsonschema.Entry
		TemplateDataKeyBodyScripts    jsonschema.Entry
		TemplateDataKeySSRScript      jsonschema.Entry
		TemplateDataKeySSRScriptHash  jsonschema.Entry
		TemplateDataKeyRootElementID  jsonschema.Entry
		ClientRootElementID           jsonschema.Entry
	}{
		IncludeDefaults:               includeDefaultsSchema,
		MainBuildEntry:                mainBuildEntrySchema,
		UIVariant:                     uiVariantSchema,
		HTMLTemplateLocation:          htmlTemplateLocationSchema,
		ClientEntry:                   clientEntrySchema,
		ClientRouteDefinitionPatterns: clientRouteDefinitionPatternsSchema,
		ServerRouteDefinitionPatterns: serverRouteDefinitionPatternsSchema,
		TSGenOutDir:                   tsGenOutDirSchema,
		BuildtimePublicURLFuncName:    buildtimePublicURLFuncNameSchema,
		DevReloadRoutesEndpointPath:   devReloadRoutesEndpointPathSchema,
		DevReloadTemplateEndpointPath: devReloadTemplateEndpointPathSchema,
		TemplateDataKeyHeadElements:   templateDataKeyHeadElementsSchema,
		TemplateDataKeyBodyScripts:    templateDataKeyBodyScriptsSchema,
		TemplateDataKeySSRScript:      templateDataKeySSRScriptSchema,
		TemplateDataKeySSRScriptHash:  templateDataKeySSRScriptHashSchema,
		TemplateDataKeyRootElementID:  templateDataKeyRootElementIDSchema,
		ClientRootElementID:           clientRootElementIDSchema,
	},
})

var includeDefaultsSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true (default), Vorma injects default watch patterns for routes, templates, and Go files.`,
	Default:     true,
})

var mainBuildEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to the Vorma build command entry point.`,
	Examples:    []string{"backend/cmd/build", "cmd/build"},
})

var uiVariantSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `The UI framework to use for client-side rendering.`,
	Enum:        []string{"react", "preact", "solid"},
})

var htmlTemplateLocationSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your HTML template file, relative to the private static directory.`,
	Examples:    []string{"entry.go.html"},
})

var clientEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your client-side entry file.`,
	Examples:    []string{"frontend/src/vorma.entry.tsx"},
})

var clientRouteDefinitionPatternsSchema = jsonschema.RequiredArray(jsonschema.Def{
	Description: `Glob patterns that resolve to client route definition files.`,
	Items:       jsonschema.RequiredString(jsonschema.Def{}),
	Examples: []string{
		"frontend/src/**/*vorma.routes.ts",
		"frontend/src/routes/core.vorma.routes.ts",
	},
})

var serverRouteDefinitionPatternsSchema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Optional backend route definition patterns to merge into the route manifest for server-only handlers.`,
	Items:       jsonschema.OptionalString(jsonschema.Def{}),
	Examples:    []string{"backend/src/**/*vorma.routes.go"},
})

var tsGenOutDirSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Directory where Vorma generates TypeScript route artifacts and filemap outputs.`,
	Examples:    []string{"frontend/src/vorma.gen"},
})

var buildtimePublicURLFuncNameSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Name of the global function injected by the Vite plugin for resolving public asset URLs at build time.`,
	Default:     "waveBuildtimeURL",
	Examples:    []string{"waveBuildtimeURL", "getAssetURL"},
})

var devReloadRoutesEndpointPathSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Dev-only endpoint path that triggers in-process route reload.`,
	Default:     "/__vorma_internal/reload-routes",
	Examples:    []string{"/__vorma_internal/reload-routes"},
})

var devReloadTemplateEndpointPathSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Dev-only endpoint path that triggers in-process template reload.`,
	Default:     "/__vorma_internal/reload-template",
	Examples:    []string{"/__vorma_internal/reload-template"},
})

var templateDataKeyHeadElementsSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for serialized head elements.`,
	Default:     "VormaHeadEls",
	Examples:    []string{"VormaHeadEls"},
})

var templateDataKeyBodyScriptsSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for serialized body script tags.`,
	Default:     "VormaBodyScripts",
	Examples:    []string{"VormaBodyScripts"},
})

var templateDataKeySSRScriptSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for the SSR bootstrap script.`,
	Default:     "VormaSSRScript",
	Examples:    []string{"VormaSSRScript"},
})

var templateDataKeySSRScriptHashSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for the SSR script CSP hash.`,
	Default:     "VormaSSRScriptSha256Hash",
	Examples:    []string{"VormaSSRScriptSha256Hash"},
})

var templateDataKeyRootElementIDSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Template data key for the root element ID placeholder.`,
	Default:     "VormaRootID",
	Examples:    []string{"VormaRootID"},
})

var clientRootElementIDSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Client root element ID used for hydration/mount.`,
	Default:     "vorma-root",
	Examples:    []string{"vorma-root"},
})
