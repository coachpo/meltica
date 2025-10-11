package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/tools/cover"
)

type stats struct {
	covered float64
	total   float64
}

func (s stats) ratio() float64 {
	if s.total == 0 {
		return 1
	}
	return s.covered / s.total
}

func main() {
	profilePath := flag.String("profile", "", "coverage profile path")
	verbose := flag.Bool("v", false, "enable verbose output")
	allowlistPath := flag.String("allowlist", "", "optional file containing package prefixes to ignore")
	flag.Parse()

	if *profilePath == "" {
		log.Fatal("--profile is required")
	}

	allowPrefixes := map[string]struct{}{}
	if *allowlistPath != "" {
		entries, err := loadAllowlist(*allowlistPath)
		if err != nil {
			log.Fatalf("load allowlist: %v", err)
		}
		for _, entry := range entries {
			allowPrefixes[entry] = struct{}{}
		}
	}

	profiles, err := cover.ParseProfiles(*profilePath)
	if err != nil {
		log.Fatalf("parse profile: %v", err)
	}
	if len(profiles) == 0 {
		log.Fatalf("coverage profile %s is empty", *profilePath)
	}

	packageStats := map[string]*stats{}

	for _, profile := range profiles {
		pkg := packageFromFile(profile.FileName)
		ps := packageStats[pkg]
		if ps == nil {
			ps = &stats{}
			packageStats[pkg] = ps
		}
		for _, block := range profile.Blocks {
			ps.total += float64(block.NumStmt)
			if block.Count > 0 {
				ps.covered += float64(block.NumStmt)
			}
		}
	}

	type rule struct {
		name      string
		threshold float64
		match     func(string) bool
	}

	rules := []rule{
		{
			name:      "core/",
			threshold: 0.90,
			match: func(pkg string) bool {
				return strings.HasPrefix(pkg, "github.com/coachpo/meltica/core/")
			},
		},
		{
			name:      "exchanges/*/routing/",
			threshold: 0.80,
			match: func(pkg string) bool {
				return strings.Contains(pkg, "/exchanges/") && strings.Contains(pkg, "/routing")
			},
		},
		{
			name:      "exchanges/*/infra/",
			threshold: 0.70,
			match: func(pkg string) bool {
				return strings.Contains(pkg, "/exchanges/") && strings.Contains(pkg, "/infra/")
			},
		},
	}

	isAllowed := func(pkg string) bool {
		for prefix := range allowPrefixes {
			if strings.HasPrefix(pkg, prefix) {
				return true
			}
		}
		return false
	}

	overall := stats{}
	for pkg, ps := range packageStats {
		if isAllowed(pkg) {
			continue
		}
		matched := false
		for _, rl := range rules {
			if rl.match(pkg) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		overall.covered += ps.covered
		overall.total += ps.total
	}
	if overall.total == 0 {
		log.Fatalf("coverage profile %s contains no executable statements", *profilePath)
	}

	const overallThreshold = 0.75
	const epsilon = 1e-6

	type violation struct {
		rule      string
		pkg       string
		coverage  float64
		threshold float64
	}

	var violations []violation

	for _, rl := range rules {
		for pkg, ps := range packageStats {
			if ps.total == 0 || !rl.match(pkg) || isAllowed(pkg) {
				continue
			}
			cov := ps.ratio()
			if cov+epsilon < rl.threshold {
				violations = append(violations, violation{rule: rl.name, pkg: pkg, coverage: cov, threshold: rl.threshold})
			}
		}
	}

	overallCov := overall.ratio()
	if overallCov+epsilon < overallThreshold {
		violations = append(violations, violation{rule: "overall", pkg: "all packages", coverage: overallCov, threshold: overallThreshold})
	}

	if *verbose {
		type summary struct {
			pkg string
			cov float64
		}
		summaries := make([]summary, 0, len(packageStats))
		for pkg, ps := range packageStats {
			summaries = append(summaries, summary{pkg: pkg, cov: ps.ratio()})
		}
		sort.Slice(summaries, func(i, j int) bool { return summaries[i].pkg < summaries[j].pkg })
		for _, s := range summaries {
			fmt.Printf("%s	%.2f%%\n", s.pkg, s.cov*100)
		}
		fmt.Printf("overall	%.2f%%\n", overallCov*100)
	}

	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "coverage threshold violations:")
		sort.Slice(violations, func(i, j int) bool {
			if violations[i].rule == violations[j].rule {
				return violations[i].pkg < violations[j].pkg
			}
			return violations[i].rule < violations[j].rule
		})
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "  %s: %s = %.2f%% (expected ≥ %.2f%%)\n", v.rule, v.pkg, v.coverage*100, v.threshold*100)
		}
		os.Exit(1)
	}
}

func packageFromFile(name string) string {
	if idx := strings.Index(name, ":"); idx >= 0 {
		name = name[:idx]
	}
	return path.Dir(name)
}

func loadAllowlist(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var entries []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
