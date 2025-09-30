package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/coachpo/meltica/internal/meltilint"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: meltilint [patterns]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./providers/..."}
	}
	ctx := context.Background()
	issues, err := meltilint.Run(ctx, patterns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "meltilint:", err)
		os.Exit(1)
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintln(os.Stderr, issue.String())
		}
		os.Exit(1)
	}
}
