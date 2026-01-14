package runtime

import (
	"fmt"

	"github.com/vormadev/vorma/lab/viteutil"
)

func (v *Vorma) getViteDevURL() string {
	if !v.GetIsDevMode() {
		return ""
	}
	return fmt.Sprintf("http://localhost:%s", viteutil.GetVitePortStr())
}
