package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/coachpo/meltica/internal/protolint"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: protolint [patterns]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./providers/..."}
	}
	ctx := context.Background()
	issues, err := protolint.Run(ctx, patterns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "protolint:", err)
		os.Exit(1)
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintln(os.Stderr, issue.String())
		}
		os.Exit(1)
	}
}
