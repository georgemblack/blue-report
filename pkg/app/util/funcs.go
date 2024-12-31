package util

import (
	"fmt"
	"hash/fnv"
)

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
