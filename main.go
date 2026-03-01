package main

import (
	"fmt"
	"os"

	"numentext/internal/app"
)

func main() {
	application := app.New()
	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
