package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/coachpo/meltica/conformance"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: validate-schemas\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Validates all JSON schemas against their golden vectors.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := conformance.ValidateSchemas(); err != nil {
		fmt.Fprintf(os.Stderr, "schema validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("All schemas validated successfully")
}
