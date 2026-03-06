// Package wavefw provides the framework-facing adapter API for Wave2.
//
// A framework adapter registers event rules and effect task roots against one
// Wave2 runtime without pushing framework-specific semantics into wave2 or
// wavebuild.
package wavefw

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/vormadev/vorma/wave2/wavebuild"
)

// RuntimeActionType is one framework-owned runtime action intent.
type RuntimeActionType string

const (
	// RuntimeActionTypeRefreshRoutes asks framework runtime to refresh route
	// definition state.
	RuntimeActionTypeRefreshRoutes RuntimeActionType = "refresh_routes"
	// RuntimeActionTypeRefreshTemplate asks framework runtime to refresh template
	// rendering shell state.
	RuntimeActionTypeRefreshTemplate RuntimeActionType = "refresh_template"
	// RuntimeActionTypeRefreshPublicFileMap asks framework runtime to refresh
	// canonical public file-map state.
	RuntimeActionTypeRefreshPublicFileMap RuntimeActionType = "refresh_public_file_map"
)

// RuntimeAction is one adapter runtime action derived from generic framework
// signals.
type RuntimeAction struct {
	Type RuntimeActionType
	// FreshnessToken is forwarded from framework signal freshness metadata for
	// stale-attempt rejection policy.
	FreshnessToken string
	// Trigger is forwarded from the originating framework signal for
	// observability.
	Trigger string
	// Metadata carries stable optional context for adapter-owned transport.
	Metadata map[string]string
}

// SignalTranslationRule maps one generic framework signal type to one
// framework runtime action type.
type SignalTranslationRule struct {
	SignalType wavebuild.FrameworkSignalType
	ActionType RuntimeActionType
}

// SignalTranslator translates generic framework signals to framework-owned
// runtime actions.
type SignalTranslator interface {
	TranslateSignals(
		signals []wavebuild.FrameworkSignal,
	) ([]RuntimeAction, error)
}

// RuleBasedSignalTranslator translates framework signals using pure-data rules.
type RuleBasedSignalTranslator struct {
	ruleBySignalType map[wavebuild.FrameworkSignalType]RuntimeActionType
}

// NewRuleBasedSignalTranslator constructs one rule-based signal translator.
func NewRuleBasedSignalTranslator(
	rules []SignalTranslationRule,
) (*RuleBasedSignalTranslator, error) {
	ruleBySignalType := make(
		map[wavebuild.FrameworkSignalType]RuntimeActionType,
		len(rules),
	)
	for _, rule := range rules {
		if rule.SignalType == "" {
			return nil, errors.New(
				"wavefw: signal translation rule signal type is required",
			)
		}
		if rule.ActionType == "" {
			return nil, fmt.Errorf(
				"wavefw: signal translation rule action type for %q is required",
				rule.SignalType,
			)
		}
		ruleBySignalType[rule.SignalType] = rule.ActionType
	}
	return &RuleBasedSignalTranslator{
		ruleBySignalType: ruleBySignalType,
	}, nil
}

// TranslateSignals translates generic framework signals to framework runtime
// actions using configured mapping rules.
func (translator *RuleBasedSignalTranslator) TranslateSignals(
	signals []wavebuild.FrameworkSignal,
) ([]RuntimeAction, error) {
	if translator == nil {
		return nil, errors.New("wavefw: signal translator is required")
	}
	if len(signals) == 0 {
		return nil, nil
	}
	actions := make([]RuntimeAction, 0, len(signals))
	seenActionKey := make(map[string]struct{}, len(signals))
	for _, signal := range signals {
		actionType, foundActionType := translator.ruleBySignalType[signal.Type]
		if !foundActionType {
			return nil, fmt.Errorf(
				"wavefw: no runtime action mapping for framework signal type %q",
				signal.Type,
			)
		}
		actionKey := string(actionType) + "|" + signal.FreshnessToken
		if _, alreadyAdded := seenActionKey[actionKey]; alreadyAdded {
			continue
		}
		seenActionKey[actionKey] = struct{}{}
		actions = append(
			actions,
			RuntimeAction{
				Type:           actionType,
				FreshnessToken: signal.FreshnessToken,
				Trigger:        signal.Trigger,
				Metadata:       cloneStringMap(signal.Metadata),
			},
		)
	}
	return actions, nil
}

var canonicalSignalTranslationRules = []SignalTranslationRule{
	{
		SignalType: wavebuild.FrameworkSignalTypeRoutesChanged,
		ActionType: RuntimeActionTypeRefreshRoutes,
	},
	{
		SignalType: wavebuild.FrameworkSignalTypeTemplateChanged,
		ActionType: RuntimeActionTypeRefreshTemplate,
	},
	{
		SignalType: wavebuild.FrameworkSignalTypePublicFileMapChanged,
		ActionType: RuntimeActionTypeRefreshPublicFileMap,
	},
}

// CanonicalSignalTranslationRules returns the baseline signal mapping policy.
func CanonicalSignalTranslationRules() []SignalTranslationRule {
	return cloneSignalTranslationRules(canonicalSignalTranslationRules)
}

// NewCanonicalSignalTranslator constructs one translator with canonical mapping
// defaults.
func NewCanonicalSignalTranslator() (*RuleBasedSignalTranslator, error) {
	return NewRuleBasedSignalTranslator(CanonicalSignalTranslationRules())
}

// Adapter describes one framework integration unit for Wave2.
type Adapter struct {
	Name             string
	EventRules       []wavebuild.EventRule
	EffectTaskRoots  []wavebuild.EffectTaskRootRegistration
	SignalTranslator SignalTranslator
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

// TranslateFrameworkSignals converts generic framework signals into
// framework-owned runtime actions using adapter translation policy.
func TranslateFrameworkSignals(
	adapter Adapter,
	signals []wavebuild.FrameworkSignal,
) ([]RuntimeAction, error) {
	translator := adapter.SignalTranslator
	if translator == nil {
		var translatorError error
		translator, translatorError = NewCanonicalSignalTranslator()
		if translatorError != nil {
			return nil, translatorError
		}
	}
	return translator.TranslateSignals(signals)
}

func cloneSignalTranslationRules(
	rules []SignalTranslationRule,
) []SignalTranslationRule {
	if len(rules) == 0 {
		return nil
	}
	cloned := make([]SignalTranslationRule, len(rules))
	copy(cloned, rules)
	return cloned
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	maps.Copy(cloned, input)
	return cloned
}
