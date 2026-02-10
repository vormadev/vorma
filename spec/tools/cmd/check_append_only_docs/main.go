package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

func extractArray(raw string, key string) ([]any, error) {
	obj := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, err
	}
	value, ok := obj[key]
	if !ok {
		return nil, fmt.Errorf("missing key %q", key)
	}
	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("key %q must be an array", key)
	}
	return arr, nil
}

func main() {
	dispatch, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	allOpen := len(dispatch.Rows) > 0
	for _, row := range dispatch.Rows {
		if row.Status != "OPEN" {
			allOpen = false
			break
		}
	}
	if allOpen {
		return
	}

	type fileRule struct {
		path string
		key  string
	}
	for _, rule := range []fileRule{
		{path: "spec/DECISIONS.json", key: "decisions"},
		{path: "spec/TRACEABILITY.json", key: "entries"},
	} {
		file := rule.path
		changed, err := specutil.IsGitFileChanged(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if !changed {
			continue
		}

		headRaw, err := specutil.GitFileAtHEAD(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "%s must exist in HEAD before append-only mode starts\n", file)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		currentRawBytes, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		headArr, err := extractArray(headRaw, rule.key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse HEAD %s: %v\n", file, err)
			os.Exit(1)
		}
		currentArr, err := extractArray(string(currentRawBytes), rule.key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse current %s: %v\n", file, err)
			os.Exit(1)
		}
		if len(currentArr) < len(headArr) {
			fmt.Fprintf(os.Stderr, "%s is append-only during mining; deletions are prohibited\n", file)
			os.Exit(1)
		}
		for i := range headArr {
			if !reflect.DeepEqual(headArr[i], currentArr[i]) {
				fmt.Fprintf(os.Stderr, "%s is append-only during mining; modifications to existing entries are prohibited\n", file)
				os.Exit(1)
			}
		}
	}
}
