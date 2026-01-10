/*
NOTE:

This package primarily exists primarily in service of internal utils.

Buyer beware.
*/
package parseutil

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/vormadev/vorma/lab/stringsutil"
)

// Returns: linesSlice, versionLineIdx, currentVersionStr
func PackageJSONFromString(content string) ([]string, int, string) {
	lines, err := stringsutil.CollectLines(content)
	if err != nil {
		panic(err)
	}
	versionLine := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), `"version":`) {
			versionLine = i
			break
		}
	}
	if versionLine == -1 {
		panic("version line not found")
	}
	versionMap := make(map[string]any)
	if err = json.Unmarshal([]byte(content), &versionMap); err != nil {
		panic(err)
	}
	currentVersion := versionMap["version"].(string)
	if currentVersion == "" {
		panic("version not found")
	}
	return lines, versionLine, currentVersion
}

// Returns: linesSlice, versionLineIdx, currentVersionStr
func PackageJSONFromFile(targetFile string) ([]string, int, string) {
	file, err := os.ReadFile(targetFile)
	if err != nil {
		panic(err)
	}
	return PackageJSONFromString(string(file))
}
