package protolint

import "go/token"

// Issue represents a single lint finding.
type Issue struct {
	Position token.Position
	Package  string
	Message  string
}

// Error implements the error interface.
func (i Issue) Error() string {
	return i.String()
}

// String formats the issue for user output.
func (i Issue) String() string {
	pos := i.Position.String()
	if i.Package != "" {
		return pos + ": [" + i.Package + "] " + i.Message
	}
	return pos + ": " + i.Message
}
