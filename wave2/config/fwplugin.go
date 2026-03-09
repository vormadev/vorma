package config

import "github.com/vormadev/vorma/lab/jsonschema"

// NOTE: in wave2, on-change hooks will run in parallel per timing setting
// (all pre hooks flighted at once, etc etc), so the order of watch include
// and exclude settings does not matter.
//
// For now, we are assuming there will be a single plugin, not multiple. In
// the future, we can support multiple with key collision protection etc if
// we really want to.

type FwPlugin = func(ctx FwPluginCtx) (FwPluginResult, error)

type FwPluginCtx struct {
	// Treat as read-only. Do not mutate unless you want trouble.
	UserConfig *Parsed
}

type FwPluginResult struct {
	// Set these relative to user's resolve root
	WatchInclude []RawIncludeEntry
	// Set these relative to user's resolve root
	WatchExclude []string // glob

	// Do not use reserved keys ("Core", "Vite", "Watch")
	JSONSchema map[string]jsonschema.Entry
}
