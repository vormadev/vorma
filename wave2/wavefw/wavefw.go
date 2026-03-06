// Package wavefw provides the framework-facing adapter API for Wave2.
//
// Framework adapters stay on the framework side and translate Wave2 framework
// signals into framework-owned refresh actions.
package wavefw

import (
	"errors"
	"fmt"

	"github.com/vormadev/vorma/wave2/wavebuild"
)

// FrameworkRefreshActionType is one framework-owned refresh action intent.
type FrameworkRefreshActionType string

const (
	// FrameworkRefreshActionTypeRefreshRoutes asks framework adapter to refresh route
	// definition state.
	FrameworkRefreshActionTypeRefreshRoutes FrameworkRefreshActionType = "refresh_routes"
	// FrameworkRefreshActionTypeRefreshTemplate asks framework adapter to refresh template
	// rendering shell state.
	FrameworkRefreshActionTypeRefreshTemplate FrameworkRefreshActionType = "refresh_template"
	// FrameworkRefreshActionTypeRefreshPublicFileMap asks framework adapter to refresh
	// canonical public file-map state.
	FrameworkRefreshActionTypeRefreshPublicFileMap FrameworkRefreshActionType = "refresh_public_file_map"
)

// FrameworkRefreshAction is one adapter action derived from generic framework
// signals.
type FrameworkRefreshAction struct {
	Type FrameworkRefreshActionType
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
// framework refresh action type.
type SignalTranslationRule struct {
	SignalType wavebuild.FrameworkSignalType
	ActionType FrameworkRefreshActionType
}

// SignalTranslator translates generic framework signals to framework-owned
// refresh actions.
type SignalTranslator interface {
	TranslateSignals(
		signals []wavebuild.FrameworkSignal,
	) ([]FrameworkRefreshAction, error)
}

// RuleBasedSignalTranslator translates framework signals using pure-data rules.
type RuleBasedSignalTranslator struct {
	ruleBySignalType map[wavebuild.FrameworkSignalType]FrameworkRefreshActionType
}

// NewRuleBasedSignalTranslator constructs one rule-based signal translator.
func NewRuleBasedSignalTranslator(
	rules []SignalTranslationRule,
) (*RuleBasedSignalTranslator, error) {
	ruleBySignalType := make(
		map[wavebuild.FrameworkSignalType]FrameworkRefreshActionType,
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

// TranslateSignals translates generic framework signals to framework refresh
// actions using configured mapping rules.
func (translator *RuleBasedSignalTranslator) TranslateSignals(
	signals []wavebuild.FrameworkSignal,
) ([]FrameworkRefreshAction, error) {
	if translator == nil {
		return nil, errors.New("wavefw: signal translator is required")
	}
	if len(signals) == 0 {
		return nil, nil
	}
	actions := make([]FrameworkRefreshAction, 0, len(signals))
	seenActionKey := make(map[string]struct{}, len(signals))
	for _, signal := range signals {
		actionType, foundActionType := translator.ruleBySignalType[signal.Type]
		if !foundActionType {
			return nil, fmt.Errorf(
				"wavefw: no framework refresh action mapping for framework signal type %q",
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
			FrameworkRefreshAction{
				Type:           actionType,
				FreshnessToken: signal.FreshnessToken,
				Trigger:        signal.Trigger,
				Metadata:       signal.Metadata,
			},
		)
	}
	return actions, nil
}

var canonicalSignalTranslationRules = []SignalTranslationRule{
	{
		SignalType: wavebuild.FrameworkSignalTypeRoutesChanged,
		ActionType: FrameworkRefreshActionTypeRefreshRoutes,
	},
	{
		SignalType: wavebuild.FrameworkSignalTypeTemplateChanged,
		ActionType: FrameworkRefreshActionTypeRefreshTemplate,
	},
	{
		SignalType: wavebuild.FrameworkSignalTypePublicFileMapChanged,
		ActionType: FrameworkRefreshActionTypeRefreshPublicFileMap,
	},
}

// CanonicalSignalTranslationRules returns the baseline signal mapping policy.
func CanonicalSignalTranslationRules() []SignalTranslationRule {
	return canonicalSignalTranslationRules
}

// NewCanonicalSignalTranslator constructs one translator with canonical mapping
// defaults.
func NewCanonicalSignalTranslator() (*RuleBasedSignalTranslator, error) {
	return NewRuleBasedSignalTranslator(CanonicalSignalTranslationRules())
}

// Adapter describes one framework integration unit for Wave2.
type Adapter struct {
	Name string
	// SignalTranslator converts Wave2 framework signals into framework-specific
	// actions.
	SignalTranslator SignalTranslator
}

// TranslateFrameworkSignals converts generic framework signals into
// framework-owned refresh actions using adapter translation policy.
func TranslateFrameworkSignals(
	adapter Adapter,
	signals []wavebuild.FrameworkSignal,
) ([]FrameworkRefreshAction, error) {
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
