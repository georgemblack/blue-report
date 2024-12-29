package util

import (
	"fmt"
	"hash/fnv"
	"os"
)

func WrapErr(message string, err error) error {
	return fmt.Errorf("%s; %w", message, err)
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func Hash(s string) string {
	hasher := fnv.New64a()
	hasher.Write([]byte(s))
	return fmt.Sprintf("%x", hasher.Sum64())
}

func GetEnvStr(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true"
}
