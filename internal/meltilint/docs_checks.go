package meltilint

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func checkDocumentationSet(root string) []Issue {
	requiredFiles := []struct {
		path     string
		stdID    string
		reason   string
		validate func(string) []Issue
	}{
		{"docs/START-HERE.md", "DOCS", "START-HERE index must exist", nil},
		{"docs/expectations/baseline-expectations/protocol-and-code-standards.md", "STD-16", "Protocol and code standards document missing", validateProtocolStandardsDoc},
		{"docs/expectations/baseline-expectations/provider-adapter-standards.md", "STD-A", "Provider adapter standards document missing", validateProviderAdapterStandardsDoc},
		{"docs/expectations/abstractions-guidelines.md", "STD-AB", "Abstractions guidelines document missing", validateAbstractionsGuidelinesDoc},
		{"docs/how-to/implementing-a-provider.md", "GUIDE", "Implementing provider guide missing", nil},
		{"docs/how-to/migration-guidelines.md", "GUIDE", "Migration guidelines missing", nil},
		{"docs/how-to/migration-steps/01-new-exchange-scaffold-and-capabilities.md", "GUIDE", "Migration step 01 missing", nil},
		{"docs/how-to/migration-steps/02-rest-surfaces-spot-and-futures.md", "GUIDE", "Migration step 02 missing", nil},
		{"docs/how-to/migration-steps/03-websocket-public-streams.md", "GUIDE", "Migration step 03 missing", nil},
		{"docs/how-to/migration-steps/04-websocket-private-streams.md", "GUIDE", "Migration step 04 missing", nil},
		{"docs/how-to/migration-steps/05-error-status-mapping-and-normalization.md", "GUIDE", "Migration step 05 missing", nil},
		{"docs/how-to/migration-steps/06-conformance-schemas-ci-release.md", "GUIDE", "Migration step 06 missing", nil},
		{"docs/validation/protocol-validation-rules.md", "RULES", "Protocol validation rules missing", validateProtocolValidationRulesDoc},
	}
	var issues []Issue
	for _, req := range requiredFiles {
		full := filepath.Join(root, req.path)
		if _, err := os.Stat(full); err != nil {
			if os.IsNotExist(err) {
				issues = append(issues, Issue{
					Position: token.Position{Filename: full},
					Message:  fmt.Sprintf("%s %s", req.stdID, req.reason),
				})
			}
			continue
		}
		if req.validate != nil {
			issues = append(issues, req.validate(full)...)
		}
	}
	return issues
}

var protocolStandardHeadingPattern = regexp.MustCompile(`(?m)^##\s+STD-(\d{2}):`)

func validateProtocolStandardsDoc(path string) []Issue {
	data, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{
			Position: token.Position{Filename: path},
			Message:  fmt.Sprintf("STD-21 failed to read protocol standards doc: %v", err),
		}}
	}
	matches := protocolStandardHeadingPattern.FindAllStringSubmatch(string(data), -1)
	expected := make(map[string]bool)
	for i := 1; i <= 21; i++ {
		expected[fmt.Sprintf("%02d", i)] = false
	}
	var unexpected []string
	for _, match := range matches {
		id := match[1]
		if _, ok := expected[id]; ok {
			expected[id] = true
		} else {
			unexpected = append(unexpected, "STD-"+id)
		}
	}
	var missing []string
	for id, seen := range expected {
		if !seen {
			missing = append(missing, "STD-"+id)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	var messages []string
	if len(missing) > 0 {
		messages = append(messages, "missing "+strings.Join(missing, ", "))
	}
	if len(unexpected) > 0 {
		messages = append(messages, "unexpected "+strings.Join(unexpected, ", "))
	}
	if len(matches) != 21 {
		messages = append(messages, fmt.Sprintf("found %d standards, expected 21", len(matches)))
	}
	if len(messages) == 0 {
		return nil
	}
	return []Issue{{
		Position: token.Position{Filename: path},
		Message:  fmt.Sprintf("STD-21 protocol-and-code-standards must enumerate STD-01 through STD-21 (%s)", strings.Join(messages, "; ")),
	}}
}

var providerStandardsHeadingPattern = regexp.MustCompile(`(?m)^##\s+STD-A(\d):`)

func validateProviderAdapterStandardsDoc(path string) []Issue {
	data, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{
			Position: token.Position{Filename: path},
			Message:  fmt.Sprintf("STD-A5 failed to read provider adapter standards doc: %v", err),
		}}
	}
	matches := providerStandardsHeadingPattern.FindAllStringSubmatch(string(data), -1)
	expected := map[string]bool{"1": false, "2": false, "3": false, "4": false, "5": false}
	var unexpected []string
	for _, match := range matches {
		id := match[1]
		if _, ok := expected[id]; ok {
			expected[id] = true
		} else {
			unexpected = append(unexpected, "STD-A"+id)
		}
	}
	var missing []string
	for id, seen := range expected {
		if !seen {
			missing = append(missing, "STD-A"+id)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	var messages []string
	if len(missing) > 0 {
		messages = append(messages, "missing "+strings.Join(missing, ", "))
	}
	if len(unexpected) > 0 {
		messages = append(messages, "unexpected "+strings.Join(unexpected, ", "))
	}
	if len(matches) != 5 {
		messages = append(messages, fmt.Sprintf("found %d adapter standards, expected 5", len(matches)))
	}
	if len(messages) == 0 {
		return nil
	}
	return []Issue{{
		Position: token.Position{Filename: path},
		Message:  fmt.Sprintf("STD-A5 provider-adapter-standards must enumerate STD-A1 through STD-A5 (%s)", strings.Join(messages, "; ")),
	}}
}

var abstractionsPhaseHeadingPattern = regexp.MustCompile(`(?m)^###\s+Phase\s+([^\n]+)`)
var phaseNumberPattern = regexp.MustCompile(`\d+`)

func validateAbstractionsGuidelinesDoc(path string) []Issue {
	data, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{
			Position: token.Position{Filename: path},
			Message:  fmt.Sprintf("STD-AB failed to read abstractions guidelines doc: %v", err),
		}}
	}
	content := string(data)
	matches := abstractionsPhaseHeadingPattern.FindAllStringSubmatch(content, -1)
	phases := make(map[int]bool)
	for _, match := range matches {
		nums := phaseNumberPattern.FindAllString(match[1], -1)
		for _, raw := range nums {
			val, err := strconv.Atoi(raw)
			if err == nil {
				phases[val] = true
			}
		}
	}
	var missing []string
	for i := 1; i <= 10; i++ {
		if !phases[i] {
			missing = append(missing, strconv.Itoa(i))
		}
	}
	var issues []Issue
	if len(missing) > 0 {
		issues = append(issues, Issue{
			Position: token.Position{Filename: path},
			Message:  fmt.Sprintf("STD-AB abstractions-guidelines must describe phases 1-10 (missing: %s)", strings.Join(missing, ", ")),
		})
	}
	if !strings.Contains(content, "Acceptance criteria per phase") {
		issues = append(issues, Issue{
			Position: token.Position{Filename: path},
			Message:  "STD-AB abstractions-guidelines must include acceptance criteria per phase section",
		})
	}
	return issues
}

var validationRulePattern = regexp.MustCompile(`^\s*(\d+)\)\s+([A-Z0-9-]+)\s+\((MUST|SHOULD|INFO)\)\s*$`)

func validateProtocolValidationRulesDoc(path string) []Issue {
	data, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{
			Position: token.Position{Filename: path},
			Message:  fmt.Sprintf("RULES failed to read protocol validation rules doc: %v", err),
		}}
	}
	lines := strings.Split(string(data), "\n")
	type ruleInfo struct {
		number        int
		id            string
		severity      string
		hasIntent     bool
		hasScope      bool
		hasValidation bool
	}
	var rules []*ruleInfo
	var current *ruleInfo
	for _, line := range lines {
		if match := validationRulePattern.FindStringSubmatch(line); match != nil {
			number, err := strconv.Atoi(match[1])
			if err != nil {
				continue
			}
			rule := &ruleInfo{number: number, id: match[2], severity: match[3]}
			rules = append(rules, rule)
			current = rule
			continue
		}
		if current == nil {
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- Intent:"):
			current.hasIntent = true
		case strings.HasPrefix(trimmed, "- Scope:"):
			current.hasScope = true
		case strings.HasPrefix(trimmed, "- Validation standard:"):
			current.hasValidation = true
		}
	}
	issuesMap := make(map[string]string)
	if len(rules) != 42 {
		issuesMap["count"] = fmt.Sprintf("found %d rules, expected 42", len(rules))
	}
	ids := make(map[string]struct{})
	numbers := make(map[int]struct{})
	var duplicateIDs []string
	for _, rule := range rules {
		if _, exists := ids[rule.id]; exists {
			duplicateIDs = append(duplicateIDs, rule.id)
		} else {
			ids[rule.id] = struct{}{}
		}
		numbers[rule.number] = struct{}{}
		missingSections := make([]string, 0, 3)
		if !rule.hasIntent {
			missingSections = append(missingSections, "Intent")
		}
		if !rule.hasScope {
			missingSections = append(missingSections, "Scope")
		}
		if !rule.hasValidation {
			missingSections = append(missingSections, "Validation standard")
		}
		if len(missingSections) > 0 {
			key := "rule:" + rule.id
			issuesMap[key] = fmt.Sprintf("rule %s missing %s", rule.id, strings.Join(missingSections, ", "))
		}
	}
	if len(duplicateIDs) > 0 {
		sort.Strings(duplicateIDs)
		issuesMap["dups"] = "duplicate rule IDs: " + strings.Join(duplicateIDs, ", ")
	}
	for i := 1; i <= 42; i++ {
		if _, ok := numbers[i]; !ok {
			key := fmt.Sprintf("missing-number-%d", i)
			issuesMap[key] = fmt.Sprintf("rule number %d missing", i)
		}
	}
	// Validate phase coverage (10 phases).
	phaseMatches := abstractionsPhaseHeadingPattern.FindAllStringSubmatch(string(data), -1)
	phases := make(map[int]struct{})
	for _, match := range phaseMatches {
		nums := phaseNumberPattern.FindAllString(match[1], -1)
		for _, raw := range nums {
			if val, err := strconv.Atoi(raw); err == nil {
				phases[val] = struct{}{}
			}
		}
	}
	for i := 1; i <= 10; i++ {
		if _, ok := phases[i]; !ok {
			key := fmt.Sprintf("missing-phase-%d", i)
			issuesMap[key] = fmt.Sprintf("phase %d heading missing", i)
		}
	}
	if len(issuesMap) == 0 {
		return nil
	}
	keys := make([]string, 0, len(issuesMap))
	for k := range issuesMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	messages := make([]string, 0, len(keys))
	for _, k := range keys {
		messages = append(messages, issuesMap[k])
	}
	return []Issue{{
		Position: token.Position{Filename: path},
		Message:  fmt.Sprintf("RULES protocol-validation-rules must enumerate 42 deterministic rules with metadata (%s)", strings.Join(messages, "; ")),
	}}
}

func checkProtocolArtifacts(root string) []Issue {
	var issues []Issue
	protocolReadme := filepath.Join(root, "protocol", "README.md")
	if _, err := os.Stat(protocolReadme); err != nil {
		if os.IsNotExist(err) {
			issues = append(issues, Issue{
				Position: token.Position{Filename: protocolReadme},
				Message:  "STD-16 protocol/README.md must exist",
			})
		}
	}
	issues = append(issues, checkSchemaCoverage(root)...)
	issues = append(issues, checkVectorCoverage(root)...)
	return issues
}

var requiredSchemaTypes = []string{
	"Instrument",
	"OrderRequest",
	"Order",
	"Position",
	"Ticker",
	"OrderBook",
	"Trade",
	"Kline",
	"TradeEvent",
	"TickerEvent",
	"DepthEvent",
	"OrderEvent",
	"BalanceEvent",
}

func checkSchemaCoverage(root string) []Issue {
	var issues []Issue
	schemaDir := filepath.Join(root, "protocol", "schemas")
	for _, typ := range requiredSchemaTypes {
		name := fmt.Sprintf("%s.schema.json", typ)
		full := filepath.Join(schemaDir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			if os.IsNotExist(err) {
				issues = append(issues, Issue{
					Position: token.Position{Filename: full},
					Message:  fmt.Sprintf("STD-17 schema %s missing", name),
				})
			}
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			issues = append(issues, Issue{
				Position: token.Position{Filename: full},
				Message:  fmt.Sprintf("STD-17 schema %s must be valid JSON: %v", name, err),
			})
			continue
		}
		if schemaVal, ok := doc["$schema"].(string); !ok || !strings.Contains(schemaVal, "2020-12") {
			issues = append(issues, Issue{
				Position: token.Position{Filename: full},
				Message:  fmt.Sprintf("STD-17 schema %s must declare Draft 2020-12", name),
			})
		}
	}
	return issues
}

func checkVectorCoverage(root string) []Issue {
	var issues []Issue
	vectorDir := filepath.Join(root, "protocol", "vectors")
	for _, typ := range requiredSchemaTypes {
		pattern := filepath.Join(vectorDir, fmt.Sprintf("%s*.json", typ))
		matches, err := filepath.Glob(pattern)
		if err != nil {
			issues = append(issues, Issue{
				Position: token.Position{Filename: pattern},
				Message:  fmt.Sprintf("STD-18 glob failure for %s: %v", typ, err),
			})
			continue
		}
		foundValid := false
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && !info.IsDir() {
				foundValid = true
				break
			}
		}
		if !foundValid {
			issues = append(issues, Issue{
				Position: token.Position{Filename: pattern},
				Message:  fmt.Sprintf("STD-18 protocol vectors for %s missing", typ),
			})
		}
	}
	return issues
}
