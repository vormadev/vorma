package main

import (
	"os"
)

func main() {
	os.Exit(run_protocol(os.Stdin, os.Stdout))
}
