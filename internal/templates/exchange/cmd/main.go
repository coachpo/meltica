package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	exch "github.com/coachpo/meltica/internal/templates/exchange"
)

func main() {
	var (
		name   = flag.String("name", "", "exchange identifier (e.g. binance)")
		output = flag.String("output", "", "output directory (defaults to ./exchanges/<name>)")
		force  = flag.Bool("force", false, "overwrite existing files")
	)
	flag.Parse()

	if *name == "" {
		log.Fatal("--name is required")
	}

	outDir := *output
	if outDir == "" {
		outDir = filepath.Join("exchanges", *name)
	}

	cfg := exch.Config{
		Name:      *name,
		OutputDir: outDir,
		Force:     *force,
	}

	if err := exch.GenerateSkeleton(cfg); err != nil {
		log.Fatalf("generate skeleton: %v", err)
	}

	fmt.Printf("Generated exchange skeleton for %s at %s\n", *name, outDir)
}
