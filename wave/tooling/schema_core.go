package tooling

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/lab/jsonschema"
)

var coreSchema = jsonschema.RequiredObject(jsonschema.Def{
	Description:      `Core Wave configuration. All paths should be set relative to the directory from which you run commands.`,
	RequiredChildren: []string{"MainAppEntry", "DistDir"},
	AllOf: []any{jsonschema.IfThen{
		If: map[string]any{
			"not": map[string]any{
				"properties": map[string]any{
					"ServerOnlyMode": map[string]any{"const": true},
				},
			},
		},
		Then: map[string]any{
			"required": []string{"StaticAssetDirs"},
		},
	}},
	Properties: struct {
		ConfigLocation                   jsonschema.Entry
		DevBuildHook                     jsonschema.Entry
		DevBuildHookTimeoutMilliseconds  jsonschema.Entry
		ProdBuildHook                    jsonschema.Entry
		ProdBuildHookTimeoutMilliseconds jsonschema.Entry
		MainAppEntry                     jsonschema.Entry
		DistDir                          jsonschema.Entry
		StaticAssetDirs                  jsonschema.Entry
		CSSEntryFiles                    jsonschema.Entry
		PublicPathPrefix                 jsonschema.Entry
		ServerOnlyMode                   jsonschema.Entry
		SequentialGoBuild                jsonschema.Entry
	}{
		ConfigLocation:                   configLocationSchema,
		DevBuildHook:                     devBuildHookSchema,
		DevBuildHookTimeoutMilliseconds:  devBuildHookTimeoutMillisecondsSchema,
		ProdBuildHook:                    prodBuildHookSchema,
		ProdBuildHookTimeoutMilliseconds: prodBuildHookTimeoutMillisecondsSchema,
		MainAppEntry:                     mainAppEntrySchema,
		DistDir:                          distDirSchema,
		StaticAssetDirs:                  staticAssetDirsSchema,
		CSSEntryFiles:                    cssEntryFilesSchema,
		PublicPathPrefix:                 publicPathPrefixSchema,
		ServerOnlyMode:                   serverOnlyModeSchema,
		SequentialGoBuild:                sequentialGoBuildSchema,
	},
})

var configLocationSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to the Wave configuration file.
This enables restarting the server when you update the Wave config.`,
	Examples: []string{"./wave.json", "./config/wave.json"},
})

var devBuildHookSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Command to run to build your app in dev mode. This runs before Wave's build process and typically generates routes or other code.`,
	Examples:    []string{"go run ./backend/cmd/build -dev", "make dev-generate"},
})

var devBuildHookTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Optional timeout in milliseconds for each dev build hook command execution (user hook and framework hook). Set to 0 or omit to disable timeout.`,
})

var prodBuildHookSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Command to run to build your app in production mode. This runs before Wave's build process and typically generates routes or other code.`,
	Examples:    []string{"go run ./backend/cmd/build", "make prod-generate"},
})

var prodBuildHookTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Optional timeout in milliseconds for each prod build hook command execution (user hook and framework hook). Set to 0 or omit to disable timeout.`,
})

var mainAppEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your app's main.go entry file (or its parent directory).`,
	Examples:    []string{"./cmd/app/main.go", "./cmd/app"},
})

var distDirSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.StaticAssetDirs.Private", "Core.StaticAssetDirs.Public") + ` This is where Wave outputs the compiled binary and processed static assets.`,
	Examples:    []string{"./dist"},
})

var staticAssetDirsSchema = jsonschema.ObjectWithOverride(`This object is required unless you are in ServerOnlyMode.
Defines where your static assets are located.`, jsonschema.Def{
	RequiredChildren: []string{"Private", "Public"},
	Properties: struct {
		Private jsonschema.Entry
		Public  jsonschema.Entry
	}{
		Private: privateSchema,
		Public:  publicSchema,
	},
})

var privateSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.DistDir", "Core.StaticAssetDirs.Public") + ` Private assets are only accessible from your Go code (e.g., templates, server-side files).`,
	Examples:    []string{"./static/private"},
})

var publicSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.DistDir", "Core.StaticAssetDirs.Private") + ` Public assets are served directly to the browser and get content-addressed hashing for cache busting. Files in a "prehashed" subdirectory will keep their original names.`,
	Examples:    []string{"./static/public"},
})

var cssEntryFilesSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Use this if you are using Wave's CSS features.
Wave will bundle and optimize your CSS files.`,
	Properties: struct {
		Critical    jsonschema.Entry
		NonCritical jsonschema.Entry
	}{
		Critical:    criticalSchema,
		NonCritical: nonCriticalSchema,
	},
})

var criticalSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your critical CSS entry file. This CSS will be inlined in the HTML head for faster initial rendering.`,
	Examples:    []string{"./styles/main.critical.css"},
})

var nonCriticalSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your non-critical CSS entry file. This CSS will be loaded asynchronously after page load.`,
	Examples:    []string{"./styles/main.css"},
})

var publicPathPrefixSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path prefix for your public assets. Must both start and end with a "/".`,
	Examples:    []string{"/public/", "/assets/", "/"},
	Default:     "/",
})

var serverOnlyModeSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, skips static asset processing/serving and browser reloading.
Use this for API-only servers without frontend assets.`,
	Default: false,
})

var sequentialGoBuildSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, the Go binary is compiled after build hooks complete rather than concurrently.
Enable this if your build hooks generate Go code that the server binary imports.`,
	Default: false,
})

var viteSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Vite integration settings. Configure these to use Vite for frontend asset bundling.`,
	Properties: struct {
		JSPackageManagerBaseCmd jsonschema.Entry
		JSPackageManagerCmdDir  jsonschema.Entry
		DefaultPort             jsonschema.Entry
		ViteConfigFile          jsonschema.Entry
	}{
		JSPackageManagerBaseCmd: jsPackageManagerBaseCmdSchema,
		JSPackageManagerCmdDir:  jsPackageManagerCmdDirSchema,
		DefaultPort:             defaultPortSchema,
		ViteConfigFile:          viteConfigFileSchema,
	},
	RequiredChildren: []string{"JSPackageManagerBaseCmd"},
})

var jsPackageManagerBaseCmdSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Base command to run Vite using your preferred package manager.
This is the command to run standalone CLIs (e.g., "npx", not "npm run").`,
	Examples: []string{"npx", "pnpm", "yarn", "bunx"},
})

var jsPackageManagerCmdDirSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Directory to run the package manager command from.
For example, if you're running commands from ".", but you want to run Vite from "./web", set this to "./web".`,
	Examples: []string{"./web", "./client"},
	Default:  ".",
})

var defaultPortSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Default port to use for Vite dev server. This is used when you run "wave dev" without specifying a port.`,
	Default:     5173,
})

var viteConfigFileSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your Vite config file if it is in a non-standard location.
Should be set relative to the JSPackageManagerCmdDir, if set, otherwise your current working directory.`,
	Examples: []string{"./configs/vite.config.ts", "vite.custom.js"},
})

func writeConfigSchema(b *Builder) error {
	schema := buildConfigSchemaRootEntry(b)

	jsonBytes, err := json.MarshalIndent(schema, "", "\t")
	if err != nil {
		return fmt.Errorf("configschema.Write: failed to marshal JSON schema: %w", err)
	}

	jsonBytes = append(jsonBytes, '\n')

	target := filepath.Join(b.cfg.Dist.Internal(), "schema.json")
	if err = os.WriteFile(target, jsonBytes, 0644); err != nil {
		return fmt.Errorf("configschema.Write: failed to write JSON schema: %w", err)
	}

	return nil
}

func buildConfigSchemaRootEntry(
	b *Builder,
) jsonschema.Entry {
	schema := jsonschema.Entry{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Type:        jsonschema.TypeObject,
		Description: "Wave configuration schema.",
		Required:    []string{"Core"},
		Properties: map[string]jsonschema.Entry{
			"Core":  coreSchema,
			"Vite":  viteSchema,
			"Watch": watchSchema,
		},
	}

	if len(b.cfg.FrameworkSchemaExtensions) == 0 {
		return schema
	}

	properties := schema.Properties.(map[string]jsonschema.Entry)
	maps.Copy(properties, b.cfg.FrameworkSchemaExtensions)
	return schema
}
