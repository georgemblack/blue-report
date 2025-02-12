package main

import (
	"fmt"
	"os"

	"github.com/georgemblack/blue-report/pkg/app/util"
)

func main() {
	fmt.Println(util.Hash(os.Args[0]))
}
