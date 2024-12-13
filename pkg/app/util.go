package app

import (
	"fmt"
	"hash/fnv"
	"os"
	"strings"
)

func wrapErr(message string, err error) error {
	return fmt.Errorf("%s; %w", message, err)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func hasPrefix(input string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}
	return false
}

func hash(s string) string {
	hasher := fnv.New64a()
	hasher.Write([]byte(s))
	return fmt.Sprintf("%x", hasher.Sum64())
}

func getEnvStr(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true"
}
