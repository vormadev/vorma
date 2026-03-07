// Package wavefw provides the framework-facing adapter API for Wave2.
//
// Framework adapters stay on the framework side and translate Wave2 framework
// notifications into framework-owned actions.
package wavefw

import (
	"errors"
	"fmt"

	"github.com/vormadev/vorma/wave2/wavebuild"
)

// FrameworkRefreshActionType is one framework-owned action intent.
type FrameworkRefreshActionType string

// FrameworkRefreshAction is one adapter action derived from generic framework
// notifications.
type FrameworkRefreshAction struct {
	Type FrameworkRefreshActionType
	// DestinationKey is forwarded from Wave notification destination key.
	DestinationKey string
	// FreshnessToken is forwarded from notification freshness metadata for
	// stale-attempt rejection policy.
	FreshnessToken string
	// Trigger is forwarded from the originating notification for
	// observability.
	Trigger string
	// Metadata carries stable optional context for adapter-owned transport.
	Metadata map[string]string
	// WaitForApp forwards app-readiness gating intent.
	WaitForApp bool
	// WaitForVite forwards Vite-readiness gating intent.
	WaitForVite bool
	// FailurePolicy forwards notification failure policy intent.
	FailurePolicy wavebuild.FrameworkNotificationFailurePolicy
}

// NotificationTranslationRule maps one generic destination key to one
// framework action type.
type NotificationTranslationRule struct {
	DestinationKey string
	ActionType     FrameworkRefreshActionType
}

// Adapter describes one framework integration unit for Wave2.
type Adapter struct {
	Name string
	// NotificationTranslationRules convert Wave2 framework notifications into
	// framework-specific action types. If no rule exists for one destination,
	// destination-key identity is used as action type.
	NotificationTranslationRules []NotificationTranslationRule
}

// TranslateFrameworkNotifications converts generic framework notifications into
// framework-owned refresh actions using adapter translation policy.
func TranslateFrameworkNotifications(
	adapter Adapter,
	notifications []wavebuild.FrameworkNotification,
) ([]FrameworkRefreshAction, error) {
	if len(notifications) == 0 {
		return nil, nil
	}
	rules := adapter.NotificationTranslationRules

	ruleBySignalType := make(
		map[string]FrameworkRefreshActionType,
		len(rules),
	)
	for _, rule := range rules {
		if rule.DestinationKey == "" {
			return nil, errors.New(
				"wavefw: notification translation rule destination key is required",
			)
		}
		if rule.ActionType == "" {
			return nil, fmt.Errorf(
				"wavefw: notification translation rule action type for %q is required",
				rule.DestinationKey,
			)
		}
		ruleBySignalType[rule.DestinationKey] = rule.ActionType
	}

	actions := make([]FrameworkRefreshAction, 0, len(notifications))
	seenActionKey := make(map[string]struct{}, len(notifications))
	for _, notification := range notifications {
		destinationKey := notification.DestinationKey()
		actionType, foundActionType := ruleBySignalType[destinationKey]
		if !foundActionType {
			actionType = FrameworkRefreshActionType(destinationKey)
		}
		if actionType == "" {
			return nil, errors.New(
				"wavefw: framework refresh action type is required",
			)
		}
		actionKey := string(actionType) + "|" + notification.FreshnessToken()
		if _, alreadyAdded := seenActionKey[actionKey]; alreadyAdded {
			continue
		}
		seenActionKey[actionKey] = struct{}{}
		actions = append(
			actions,
			FrameworkRefreshAction{
				Type:           actionType,
				DestinationKey: destinationKey,
				FreshnessToken: notification.FreshnessToken(),
				Trigger:        notification.Trigger(),
				Metadata:       notification.Metadata(),
				WaitForApp:     notification.WaitForApp(),
				WaitForVite:    notification.WaitForVite(),
				FailurePolicy:  notification.FailurePolicy(),
			},
		)
	}
	return actions, nil
}
