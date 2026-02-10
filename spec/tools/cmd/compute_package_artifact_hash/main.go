package main

import (
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./spec/tools/cmd/compute_package_artifact_hash <spec/packages/path>")
		os.Exit(1)
	}

	hash, err := specutil.ComputeSpecHash(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "compute hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
