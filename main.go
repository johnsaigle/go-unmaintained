package main

import (
	"fmt"
	"os"

	"github.com/johnsaigle/go-unmaintained/cmd"
)

func main() {

	var i *int
	add(i)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}

func add(i *int) int {
	return 1 + *i
}
