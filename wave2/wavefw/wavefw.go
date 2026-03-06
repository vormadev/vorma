// Package wavefw provides the framework-facing adapter API for Wave2.
//
// A framework adapter registers event rules and effect task roots against one
// Wave2 runtime without pushing framework-specific semantics into wave2 or
// wavebuild.
package wavefw

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vormadev/vorma/wave2/wavebuild"
)

// Adapter describes one framework integration unit for Wave2.
type Adapter struct {
	Name            string
	EventRules      []wavebuild.EventRule
	EffectTaskRoots []wavebuild.EffectTaskRootRegistration
}

// Register attaches one framework adapter to one Wave2 build engine.
func Register(engine *wavebuild.Engine, adapter Adapter) error {
	if engine == nil {
		return errors.New("wavefw: wavebuild engine is required")
	}
	adapterName := strings.TrimSpace(adapter.Name)
	if adapterName == "" {
		return errors.New("wavefw: adapter name is required")
	}
	if registerRulesError := engine.RegisterEventRules(
		adapter.EventRules...,
	); registerRulesError != nil {
		return fmt.Errorf(
			"wavefw: register event rules for adapter %q: %w",
			adapterName,
			registerRulesError,
		)
	}
	if registerTaskRootsError := engine.RegisterEffectTaskRoots(
		adapter.EffectTaskRoots...,
	); registerTaskRootsError != nil {
		return fmt.Errorf(
			"wavefw: register effect task roots for adapter %q: %w",
			adapterName,
			registerTaskRootsError,
		)
	}
	return nil
}
