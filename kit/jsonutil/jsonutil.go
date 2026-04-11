package jsonutil

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSONString is a semantic marker type for JSON payload strings.
type JSONString string

// Serializes a value to JSON using json.Marshal.
func Serialize(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("error encoding JSON: %w", err)
	}
	return data, nil
}

// Serializes with json.MarshalIndent using tabs.
func SerializePretty(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("error encoding JSON: %w", err)
	}
	return data, nil
}

// Serializes a value to JSON and writes it to a file.
// Defaults to 0644 permissions if not specified.
func SerializeToFile(path string, v any, fm ...os.FileMode) error {
	return serialize_to_file(path, v, Serialize, fm...)
}

// Serializes a value to JSON and writes it to a file with pretty formatting.
// Defaults to 0644 permissions if not specified.
func SerializePrettyToFile(path string, v any, fm ...os.FileMode) error {
	return serialize_to_file(path, v, SerializePretty, fm...)
}

// Parses JSON data into a value of type T.
func Parse[T any](data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("error decoding JSON: %w", err)
	}
	return v, nil
}

// Reads a JSON file and parses its contents into a value of type T.
func ParseFromFile[T any](path string) (T, error) {
	var v T
	contents, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	return Parse[T](contents)
}

func serialize_to_file(
	path string,
	v any,
	serializer func(v any) ([]byte, error),
	fm ...os.FileMode,
) error {
	data, err := serializer(v)
	if err != nil {
		return err
	}
	if len(fm) > 0 {
		return os.WriteFile(path, data, fm[0])
	}
	return os.WriteFile(path, data, 0644)
}
