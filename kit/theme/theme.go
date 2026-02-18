package theme

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
)

const (
	SystemValue             = "system"
	LightValue              = "light"
	DarkValue               = "dark"
	themeCookieName         = "kit_theme"
	resolvedThemeCookieName = "kit_resolved_theme"
)

type ThemeData struct {
	Theme                 string
	ResolvedTheme         string
	ResolvedThemeOpposite string
	HTMLClass             string
}

func GetThemeData(r *http.Request) ThemeData {
	if r == nil {
		return ThemeData{
			Theme:                 SystemValue,
			ResolvedTheme:         LightValue,
			ResolvedThemeOpposite: DarkValue,
			HTMLClass:             "system light",
		}
	}

	rawTheme := SystemValue
	c, err := r.Cookie(themeCookieName)
	if err == nil {
		rawTheme = normalizeTheme(c.Value)
	}

	resolvedTheme := rawTheme

	htmlClass := strings.Builder{}
	htmlClass.WriteString(rawTheme)

	if rawTheme == SystemValue {
		resolvedTheme = getResolved(r)
		htmlClass.WriteString(" ")
		htmlClass.WriteString(resolvedTheme)
	}

	return ThemeData{
		Theme:                 rawTheme,
		ResolvedTheme:         resolvedTheme,
		ResolvedThemeOpposite: getResolvedOpposite(resolvedTheme),
		HTMLClass:             htmlClass.String(),
	}
}

func getResolved(r *http.Request) string {
	if r == nil {
		return LightValue
	}
	c, err := r.Cookie(resolvedThemeCookieName)
	if err != nil {
		return LightValue
	}
	return normalizeResolvedTheme(c.Value)
}

func getResolvedOpposite(theme string) string {
	if theme == LightValue {
		return DarkValue
	}
	return LightValue
}

func normalizeTheme(value string) string {
	switch value {
	case SystemValue, LightValue, DarkValue:
		return value
	default:
		return SystemValue
	}
}

func normalizeResolvedTheme(value string) string {
	switch value {
	case DarkValue:
		return DarkValue
	default:
		return LightValue
	}
}

var systemThemeScript, systemThemeScriptSha256Hash = mustGetSystemThemeScript()

func GetSystemThemeScript() template.HTML {
	return systemThemeScript
}

func GetSystemThemeScriptSha256Hash() string {
	return systemThemeScriptSha256Hash
}

func mustGetSystemThemeScript() (template.HTML, string) {
	el := &htmlutil.Element{Tag: "script", DangerousInnerHTML: string(systemThemeScriptInnerHTML)}
	sha256Hash, err := htmlutil.ComputeContentSha256(el)
	if err != nil {
		panic("could not compute CSP hash for system theme script: " + err.Error())
	}
	renderedEl, err := htmlutil.RenderElement(el)
	if err != nil {
		panic("could not render system theme script element: " + err.Error())
	}
	return renderedEl, sha256Hash
}

const systemThemeScriptInnerHTML = template.HTML(`
if (window.document.documentElement.classList.contains("system")) {
	const isDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
	window.document.documentElement.classList.add(isDark ? "dark" : "light");
	window.document.documentElement.classList.remove(isDark ? "light" : "dark");
}
`)
