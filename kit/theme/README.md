# kit/theme

`github.com/vormadev/vorma/kit/theme`

Theme helper for server-rendered apps with `light` / `dark` / `system` modes.

It provides:

- normalized theme data from cookies
- ready-to-use HTML class string
- inline script + CSP hash for system-theme flash prevention

## Import

```go
import "github.com/vormadev/vorma/kit/theme"
```

## Quick Start

```go
td := theme.GetThemeData(r)
```

Use `td.HTMLClass` on `<html>`:

```html
<html class="{{.Theme.HTMLClass}}">
```

Inject the system script early in `<head>`:

```go
theme.SystemThemeScript
```

If you use CSP, include:

```go
theme.SystemThemeScriptSha256Hash
```

## Theme Values

- `SystemValue` (`"system"`)
- `LightValue` (`"light"`)
- `DarkValue` (`"dark"`)

`GetThemeData` returns:

- raw user preference (`Theme`)
- resolved concrete theme (`ResolvedTheme`)
- opposite concrete theme (`ResolvedThemeOpposite`)
- HTML class string (`HTMLClass`)

Invalid or unexpected cookie values are normalized safely:

- invalid `Theme` -> `system`
- invalid resolved-theme cookie -> `light`

Cookie names are fixed internally:

- `kit_theme`
- `kit_resolved_theme`

Plan integrations around those names (or wrap this package if custom names are required).

## API Coverage

### Constants

- `const DarkValue`
- `const LightValue`
- `const SystemValue`

### Variables

- `var SystemThemeScript`
- `var SystemThemeScriptSha256Hash`

### Types

- `type ThemeData`

### Exported Struct Fields

- `ThemeData.HTMLClass string`
- `ThemeData.ResolvedTheme string`
- `ThemeData.ResolvedThemeOpposite string`
- `ThemeData.Theme string`

### Functions

- `func GetThemeData(r *http.Request) ThemeData`
