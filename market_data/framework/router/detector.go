package router

import (
	"errors"
	"fmt"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/errs"
)

// FieldDetectionStrategy inspects a JSON field to determine the message type identifier.
type FieldDetectionStrategy struct {
	messageTypeID string
	fieldParts    []string
	expectedValue string
}

// NewFieldDetectionStrategy constructs a strategy for the supplied rule.
func NewFieldDetectionStrategy(rule DetectionRule, messageTypeID string) (*FieldDetectionStrategy, error) {
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	parts := splitPath(rule.FieldPath)
	if len(parts) == 0 {
		return nil, newDetectionError("field path required", nil, true)
	}
	return &FieldDetectionStrategy{
		messageTypeID: messageTypeID,
		fieldParts:    parts,
		expectedValue: strings.TrimSpace(rule.ExpectedValue),
	}, nil
}

// Detect returns the configured message type identifier when the field matches the expected value.
func (s *FieldDetectionStrategy) Detect(raw []byte) (string, error) {
	if s == nil {
		return "", newDetectionError("field detection strategy not configured", nil, false)
	}
	actual, err := s.extract(raw)
	if err != nil {
		return "", err
	}
	if actual != s.expectedValue {
		return "", newDetectionError("field value did not match expected", nil, true)
	}
	return s.messageTypeID, nil
}

func (s *FieldDetectionStrategy) extract(raw []byte) (string, error) {
	var cursor json.RawMessage = raw
	for idx, part := range s.fieldParts {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(cursor, &envelope); err != nil {
			return "", newDetectionError("field detection parse failure", err, false)
		}
		field, ok := envelope[part]
		if !ok {
			return "", newDetectionError(fmt.Sprintf("field %s not found", part), nil, true)
		}
		if idx == len(s.fieldParts)-1 {
			var value string
			if err := json.Unmarshal(field, &value); err != nil {
				return "", newDetectionError(fmt.Sprintf("field %s decode failure", part), err, false)
			}
			return strings.TrimSpace(value), nil
		}
		cursor = field
	}
	return "", newDetectionError("field path resolution failed", nil, true)
}

func splitPath(path string) []string {
	segments := strings.Split(strings.TrimSpace(path), ".")
	result := make([]string, 0, len(segments))
	for _, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

type detectionError struct {
	err     *errs.E
	noMatch bool
}

func (e *detectionError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *detectionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newDetectionError(message string, cause error, noMatch bool) error {
	opts := []errs.Option{errs.WithMessage(message)}
	if cause != nil {
		opts = append(opts, errs.WithCause(cause))
	}
	return &detectionError{err: errs.New("", errs.CodeInvalid, opts...), noMatch: noMatch}
}

func isNoMatchError(err error) bool {
	var detErr *detectionError
	if errors.As(err, &detErr) {
		return detErr.noMatch
	}
	return false
}
