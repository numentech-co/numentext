package main

import (
	"fmt"
	"os"

	"numentext/internal/app"
)

func main() {
	application := app.New()
	// Open files passed as command line arguments
	for _, arg := range os.Args[1:] {
		application.OpenFileByPath(arg)
	}
	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
