package app

import (
	"bufio"
	"embed"

	mapset "github.com/deckarep/golang-set/v2"
)

//go:embed assets/news-hosts.txt
var listSource embed.FS

func GetNewsHosts() (mapset.Set[string], error) {
	file, err := listSource.Open("assets/news-hosts.txt")
	if err != nil {
		return nil, wrapErr("failed to open news-hosts.txt", err)
	}
	defer file.Close()

	hosts := mapset.NewSet[string]()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		hosts.Add(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, wrapErr("error when scanning news-hosts.txt", err)
	}

	return hosts, nil
}
