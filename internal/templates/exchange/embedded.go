package exchange

import "embed"

// templateFS holds the exchange skeleton templates.
//
//go:embed templates/*/*.tmpl templates/*/*/*.tmpl
var templateFS embed.FS
