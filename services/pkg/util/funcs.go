package util

import (
	"fmt"
	"hash/fnv"
)

func ContainsStr(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func ContainsInt(i []int, e int) bool {
	for _, a := range i {
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
