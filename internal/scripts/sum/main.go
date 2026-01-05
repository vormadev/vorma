package main

import (
	"log"
	"os"

	"github.com/vormadev/vorma/kit/lab/repoconcat"
)

func main() {
	// create dir called __LLM_CONCAT.local if it doesn't exist
	if _, err := os.Stat("__LLM_CONCAT.local"); os.IsNotExist(err) {
		os.Mkdir("__LLM_CONCAT.local", 0755)
	}

	/////// ALL
	if err := repoconcat.Concat(repoconcat.Config{
		Root:   ".",
		Output: "__LLM_CONCAT.local/ALL.local.txt",
		IgnoreDirs: []string{
			".hooks",
			"**/dist",
			"npm_dist",
			"**/*.local",
			"**/*.local.*",
			"internal/junk-drawer",
			"internal/scripts",
			"internal/site",
			"kit/lab",

			//maybe
			"wave",
			"kit/_typescript",
			"bootstrap",
		},
		IgnoreFiles: []string{
			"CODE_OF_CONDUCT.md",
			"LICENSE",
			"RELEASE_INSTRUCTIONS.md",
			".oxlintrc.json",
			"**/*_test.go",
			"**/*.test.*",
			"**/bench.txt",
			"**/*.bench.*",
			"**/*.local",
			"**/*.local.*",
			"**/*.bench.txt",
			"**/package.json",
			"**/tsconfig.json",
			"**/tsconfig.*.json",
			"vitest.config.ts",
		},
		Verbose: true,
	}); err != nil {
		log.Fatal(err)
	}

	// /////// SITE
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:   "./internal/site",
	// 	Output: "__LLM_CONCAT.local/INTERNAL_SITE.local.txt",
	// 	IgnoreDirs: []string{
	// 		"backend/dist",
	// 		"backend/assets/markdown",
	// 	},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// VORMA FRAMEWORK
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:   "./internal/framework",
	// 	Output: "__LLM_CONCAT.local/INTERNAL_FRAMEWORK.local.txt",
	// 	IgnoreDirs: []string{
	// 		"_typescript/create",
	// 	},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// WAVE
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./wave",
	// 	Output:     "LLM__WAVE.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// KIT -- MATCHER
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./kit/matcher",
	// 	Output:     "LLM__KIT_MATCHER.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		// "**/*_test.go",
	// 		// "**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// KIT -- VALIDATE
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./kit/validate",
	// 	Output:     "LLM__KIT_VALIDATE.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// KIT -- TASKS
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./kit/tasks",
	// 	Output:     "LLM__KIT_TASKS.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// KIT -- MUX
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./kit/mux",
	// 	Output:     "LLM__KIT_MUX.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// KIT -- RESPONSE
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./kit/response",
	// 	Output:     "LLM__KIT_RESPONSE.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// KIT -- VITEUTIL
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./kit/viteutil",
	// 	Output:     "LLM__KIT_VITEUTIL.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// BOOTSTRAPPER -- GO CORE
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./bootstrap",
	// 	Output:     "LLM__BOOTSTRAP.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// BOOTSTRAPPER -- NPM PACKAGE SCRIPT
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Root:       "./internal/framework/_typescript/create",
	// 	Output:     "LLM__CREATE_TS.local.txt",
	// 	IgnoreDirs: []string{},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }

	// /////// KIT -- PRIMITIVES FOR LLMs
	// if err := repoconcat.Concat(repoconcat.Config{
	// 	Output: "__LLM_CONCAT.local/KIT_PRIMITIVES.local.txt",
	// 	IncludeDirs: []string{
	// 		"./kit/id",
	// 		"./kit/keyset",
	// 		"./kit/securebytes",
	// 		"./kit/grace",
	// 		"./kit/lazycache",
	// 		"./kit/lazyget",
	// 		"./kit/safecache",
	// 		"./kit/tasks",
	// 		"./kit/bytesutil",
	// 		"./kit/cryptoutil",
	// 		"./kit/cookies",
	// 		"./kit/csrf",
	// 	},
	// 	IgnoreFiles: []string{
	// 		"**/*_test.go",
	// 		"**/*.test.*",
	// 		"bench.txt",
	// 		"**/*.bench.*",
	// 		"**/*.local.*",
	// 		"**/*.bench.txt",
	// 	},
	// 	Verbose: true,
	// }); err != nil {
	// 	log.Fatal(err)
	// }
}
