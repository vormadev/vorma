package theme

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetThemeDataDefaults(t *testing.T) {
	data := GetThemeData(nil)
	if data.Theme != SystemValue {
		t.Fatalf("expected default theme %q, got %q", SystemValue, data.Theme)
	}
	if data.ResolvedTheme != LightValue {
		t.Fatalf("expected default resolved theme %q, got %q", LightValue, data.ResolvedTheme)
	}
	if data.HTMLClass != "system light" {
		t.Fatalf("expected default html class %q, got %q", "system light", data.HTMLClass)
	}
}

func TestGetThemeDataInvalidCookieValuesAreNormalized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: themeCookieName, Value: "unexpected"})
	req.AddCookie(&http.Cookie{Name: resolvedThemeCookieName, Value: "hacker-value"})

	data := GetThemeData(req)
	if data.Theme != SystemValue {
		t.Fatalf("expected normalized theme %q, got %q", SystemValue, data.Theme)
	}
	if data.ResolvedTheme != LightValue {
		t.Fatalf("expected normalized resolved theme %q, got %q", LightValue, data.ResolvedTheme)
	}
	if data.HTMLClass != "system light" {
		t.Fatalf("expected normalized html class %q, got %q", "system light", data.HTMLClass)
	}
}

func TestGetThemeDataRespectsValidDarkCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: themeCookieName, Value: DarkValue})

	data := GetThemeData(req)
	if data.Theme != DarkValue {
		t.Fatalf("expected theme %q, got %q", DarkValue, data.Theme)
	}
	if data.ResolvedTheme != DarkValue {
		t.Fatalf("expected resolved theme %q, got %q", DarkValue, data.ResolvedTheme)
	}
	if data.ResolvedThemeOpposite != LightValue {
		t.Fatalf("expected opposite theme %q, got %q", LightValue, data.ResolvedThemeOpposite)
	}
	if data.HTMLClass != "dark" {
		t.Fatalf("expected html class %q, got %q", "dark", data.HTMLClass)
	}
}
