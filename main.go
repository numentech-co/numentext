package main

import (
	"fmt"
	"os"

	"numentext/internal/app"
)

func main() {
	// Check for --merge flag: numentext --merge LOCAL BASE REMOTE MERGED
	if len(os.Args) >= 6 && os.Args[1] == "--merge" {
		localPath := os.Args[2]
		basePath := os.Args[3]
		remotePath := os.Args[4]
		mergedPath := os.Args[5]

		application := app.New()
		if err := application.OpenMergeFiles(localPath, basePath, remotePath, mergedPath); err != nil {
			fmt.Fprintf(os.Stderr, "Merge error: %v\n", err)
			os.Exit(1)
		}
		application.StartMergeMode()
		if err := application.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

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
