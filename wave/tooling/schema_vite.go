package tooling

import "github.com/vormadev/vorma/lab/jsonschema"

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
