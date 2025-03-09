package testutil

import (
	"embed"
	"fmt"
)

//go:embed test-data/*
var files embed.FS

func GetTestData(name string) []byte {
	bytes, _ := files.ReadFile(fmt.Sprintf("test-data/%s", name))
	return bytes
}
