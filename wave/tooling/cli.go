package tooling

import (
	"flag"
	"log/slog"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/wave"
)

// BuildWaveWithHook provides CLI integration for build commands with a custom hook.
func BuildWaveWithHook(cfg *wave.ParsedConfig, log *slog.Logger, hook func(isDev bool) error) {
	if log == nil {
		log = colorlog.New("wave")
	}

	devMode := flag.Bool("dev", false, "run in dev mode")
	hookMode := flag.Bool("hook", false, "run custom hook only")
	noBinary := flag.Bool("no-binary", false, "skip go binary compilation")
	flag.Parse()

	if *hookMode && hook != nil {
		if err := hook(*devMode); err != nil {
			panic(err)
		}
		return
	}

	if *devMode {
		if err := RunDev(cfg, log); err != nil {
			panic(err)
		}
		return
	}

	b := NewBuilder(cfg, log)
	defer b.Close()

	if err := b.Build(BuildOpts{CompileGo: !*noBinary}); err != nil {
		panic(err)
	}
}

// BuildWave is a simplified entry point without a custom hook.
func BuildWave(cfg *wave.ParsedConfig, log *slog.Logger) {
	if log == nil {
		log = colorlog.New("wave")
	}

	BuildWaveWithHook(cfg, log, nil)
}
