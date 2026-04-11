package npm

import (
	_ "embed"
	"fmt"

	"github.com/vormadev/vorma/kit/jsonutil"
)

type package_json struct {
	Version string `json:"version"`
}

//go:embed package.json
var pkg_json_bytes []byte

func Version() (string, error) {
	package_json, err := jsonutil.Parse[package_json](pkg_json_bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse package.json: %w", err)
	}
	return package_json.Version, nil
}
