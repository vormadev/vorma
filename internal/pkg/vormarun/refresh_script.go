package vormarun

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
)

//go:embed refresh_script.js
var refresh_script_tmpl string

// DevRefreshScriptContentSha256 returns the base64-encoded SHA-256 hash
// of the refresh script content, suitable for Content-Security-Policy
// script-src directives. Returns empty string if not in dev mode.
func (inst *Instance) DevRefreshScriptContentSha256() (string, error) {
	if !IsDev() {
		return "", nil
	}
	if err := inst.init(); err != nil {
		return "", fmt.Errorf("error initializing instance: %w", err)
	}
	inner_html, error := inst.refresh_script_inner_html()
	if error != nil {
		return "", fmt.Errorf("error getting refresh script inner HTML for hashing: %w", error)
	}
	return bytesutil.ToBase64(cryptoutil.Sha256Hash([]byte(inner_html))), nil
}

func (inst *Instance) refresh_script_inner_html() (string, error) {
	if !IsDev() {
		return "", nil
	}
	manifest, err := inst.manifest()
	if err != nil {
		return "", fmt.Errorf("error getting manifest for refresh script: %w", err)
	}
	script := refresh_script_tmpl
	script = strings.ReplaceAll(script,
		"__REPLACE_ME_WITH_REFRESH_PORT__", fmt.Sprintf("%d", manifest.Dev_MuxPort),
	)
	script = strings.ReplaceAll(script,
		"__REPLACE_ME_WITH_REFRESH_TOKEN__", manifest.Dev_RefreshToken,
	)
	return "\n" + script, nil
}
