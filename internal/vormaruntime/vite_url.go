package vormaruntime

import (
	"fmt"

	"github.com/vormadev/vorma/lab/viteutil"
)

func (v *Vorma) getViteDevURL() string {
	return getViteDevURLForMode(v.GetIsDevMode())
}

func getViteDevURLForMode(isDevMode bool) string {
	if !isDevMode {
		return ""
	}
	return fmt.Sprintf("http://localhost:%s", viteutil.GetVitePortStr())
}
