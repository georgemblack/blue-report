package main

import (
	"fmt"
	"os"

	"github.com/georgemblack/blue-report/pkg/util"
)

func main() {
	fmt.Println(util.Hash(os.Args[0]))
}
