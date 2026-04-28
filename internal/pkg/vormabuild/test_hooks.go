package vormabuild

import (
	"time"

	"github.com/vormadev/vorma/kit/envutil"
)

func (rs *run_state) panic_if_test_stage(stage string) {
	if envutil.GetStr(__test_panic_stage_env_key, "") == stage {
		panic("test panic: " + stage)
	}
}

func (rs *run_state) panic_async_if_test_stage(stage string) {
	if envutil.GetStr(__test_panic_stage_env_key, "") != stage {
		return
	}
	rs.go_safely(func() {
		time.Sleep(100 * time.Millisecond)
		panic("test async panic: " + stage)
	})
}
