//go:build !prod

package backend

import "github.com/vormadev/vorma/wave"

var Wave = wave.New(wave.Options{})
