package appsupervisor

import (
	"net/http"
	"testing"
	"time"
)

func TestNormalizeReadinessWaitPolicy_PreservesCustomFieldsWhenTimeoutIsUnset(
	t *testing.T,
) {
	customPolicy := ReadinessWaitPolicy{
		HTTPClientTimeout: 0,
		InitialDelay:      7 * time.Millisecond,
		MaximumDelay:      11 * time.Millisecond,
		MaximumTotalWait:  19 * time.Millisecond,
		TreatHTTPStatusCodeAsReady: func(statusCode int) bool {
			return statusCode == http.StatusNoContent
		},
	}

	normalizedPolicy := normalizeReadinessWaitPolicy(customPolicy)
	defaultPolicy := DefaultReadinessWaitPolicy()

	if normalizedPolicy.HTTPClientTimeout != defaultPolicy.HTTPClientTimeout {
		t.Fatalf(
			"expected default HTTP client timeout %v, got %v",
			defaultPolicy.HTTPClientTimeout,
			normalizedPolicy.HTTPClientTimeout,
		)
	}
	if normalizedPolicy.InitialDelay != customPolicy.InitialDelay {
		t.Fatalf(
			"expected InitialDelay %v, got %v",
			customPolicy.InitialDelay,
			normalizedPolicy.InitialDelay,
		)
	}
	if normalizedPolicy.MaximumDelay != customPolicy.MaximumDelay {
		t.Fatalf(
			"expected MaximumDelay %v, got %v",
			customPolicy.MaximumDelay,
			normalizedPolicy.MaximumDelay,
		)
	}
	if normalizedPolicy.MaximumTotalWait != customPolicy.MaximumTotalWait {
		t.Fatalf(
			"expected MaximumTotalWait %v, got %v",
			customPolicy.MaximumTotalWait,
			normalizedPolicy.MaximumTotalWait,
		)
	}
	if !normalizedPolicy.TreatHTTPStatusCodeAsReady(http.StatusNoContent) {
		t.Fatal("expected custom readiness predicate to treat 204 as ready")
	}
	if normalizedPolicy.TreatHTTPStatusCodeAsReady(http.StatusOK) {
		t.Fatal("expected custom readiness predicate to reject 200")
	}
}

func TestNormalizeReadinessWaitPolicy_FillsUnsetFieldsWithDefaults(
	t *testing.T,
) {
	normalizedPolicy := normalizeReadinessWaitPolicy(ReadinessWaitPolicy{})
	defaultPolicy := DefaultReadinessWaitPolicy()

	if normalizedPolicy.HTTPClientTimeout != defaultPolicy.HTTPClientTimeout {
		t.Fatalf(
			"expected HTTPClientTimeout default %v, got %v",
			defaultPolicy.HTTPClientTimeout,
			normalizedPolicy.HTTPClientTimeout,
		)
	}
	if normalizedPolicy.InitialDelay != defaultPolicy.InitialDelay {
		t.Fatalf(
			"expected InitialDelay default %v, got %v",
			defaultPolicy.InitialDelay,
			normalizedPolicy.InitialDelay,
		)
	}
	if normalizedPolicy.MaximumDelay != defaultPolicy.MaximumDelay {
		t.Fatalf(
			"expected MaximumDelay default %v, got %v",
			defaultPolicy.MaximumDelay,
			normalizedPolicy.MaximumDelay,
		)
	}
	if normalizedPolicy.MaximumTotalWait != defaultPolicy.MaximumTotalWait {
		t.Fatalf(
			"expected MaximumTotalWait default %v, got %v",
			defaultPolicy.MaximumTotalWait,
			normalizedPolicy.MaximumTotalWait,
		)
	}
	if !normalizedPolicy.TreatHTTPStatusCodeAsReady(http.StatusOK) {
		t.Fatal("expected default readiness predicate to treat 200 as ready")
	}
	if normalizedPolicy.TreatHTTPStatusCodeAsReady(http.StatusFound) {
		t.Fatal("expected default readiness predicate to reject 302")
	}
	if normalizedPolicy.TreatHTTPStatusCodeAsReady(
		http.StatusInternalServerError,
	) {
		t.Fatal("expected default readiness predicate to reject 500")
	}
}
