---
title: theme
description:
    Light/dark/system theme support via cookies with flash-prevention script.
---

```go
import "github.com/vormadev/vorma/kit/theme"
```

## Constants

```go
const SystemValue = "system"
const LightValue = "light"
const DarkValue = "dark"
```

## ThemeData

```go
type ThemeData struct {
    Theme                 string  // "system", "light", or "dark"
    ResolvedTheme         string  // "light" or "dark" (resolved from system preference)
    ResolvedThemeOpposite string  // opposite of ResolvedTheme
    HTMLClass             string  // for <html class="..."> e.g. "system light"
}

func GetThemeData(r *http.Request) ThemeData
```

## Flash Prevention Script

Inline script to apply resolved theme before paint (prevents flash of wrong
theme):

```go
var SystemThemeScript template.HTML           // rendered <script>...</script>
var SystemThemeScriptSha256Hash string        // for CSP header
```

## Cookie Names

- `kit_theme`: User preference ("system", "light", "dark")
- `kit_resolved_theme`: System-resolved value when preference is "system"

## Example

```go
// In handler
data := theme.GetThemeData(r)

// In template
<html class="{{.HTMLClass}}">
<head>
    {{.SystemThemeScript}}  <!-- prevents flash -->
</head>
```
