package main

import (
	"context"

	"github.com/vormadev/vorma/wave4"
)

func main() {
	wave4.Super(wave4.SuperOptions{
		ParentContext:         context.Background(),
		CWDRelativeConfigPath: "./wave4/cmd/tester/wave.config.json",
	})
}
