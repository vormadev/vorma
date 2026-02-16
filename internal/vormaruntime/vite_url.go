package vormaruntime

import (
	"fmt"

	"github.com/vormadev/vorma/lab/viteutil"
)

func getViteDevURLForMode(isDevMode bool) string {
	if !isDevMode {
		return ""
	}
	return fmt.Sprintf("http://localhost:%s", viteutil.GetVitePortStr())
}
