package envutil

import (
	"os"
	"strconv"
)

func GetStr(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func GetInt(key string, defaultValue int) int {
	strValue := GetStr(key, strconv.Itoa(defaultValue))
	value, err := strconv.Atoi(strValue)
	if err == nil {
		return value
	}
	return defaultValue
}

func GetBool(key string, defaultValue bool) bool {
	strValue := GetStr(key, strconv.FormatBool(defaultValue))
	value, err := strconv.ParseBool(strValue)
	if err == nil {
		return value
	}
	return defaultValue
}
