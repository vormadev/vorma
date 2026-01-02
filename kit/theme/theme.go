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
	c, err := r.Cookie(themeCookieName)
	if err != nil {
		return ThemeData{
			Theme:                 SystemValue,
			ResolvedTheme:         LightValue,
			ResolvedThemeOpposite: DarkValue,
			HTMLClass:             "system light",
		}
	}

	rawTheme := c.Value
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
	c, err := r.Cookie(resolvedThemeCookieName)
	if err != nil {
		return LightValue
	}
	return c.Value
}

func getResolvedOpposite(theme string) string {
	if theme == LightValue {
		return DarkValue
	}
	return LightValue
}

var SystemThemeScript, SystemThemeScriptSha256Hash = mustGetSystemThemeScript()

func mustGetSystemThemeScript() (template.HTML, string) {
	el := &htmlutil.Element{Tag: "script", DangerousInnerHTML: string(systemThemeScriptInnerHTML)}
	sha256Hash, _ := htmlutil.AddSha256HashInline(el)
	renderedEl, _ := htmlutil.RenderElement(el)
	return renderedEl, sha256Hash
}

const systemThemeScriptInnerHTML = template.HTML(`
if (window.document.documentElement.classList.contains("system")) {
	const isDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
	window.document.documentElement.classList.add(isDark ? "dark" : "light");
	window.document.documentElement.classList.remove(isDark ? "light" : "dark");
}
`)
