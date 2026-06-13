// Package detection provides compiled text detectors for KiwiGuard policy rules.
package detection

import (
	"crypto/sha256"
	"encoding/hex"
)

// Kind identifies a detector implementation.
type Kind string

// Direction identifies which side of LLM traffic is being inspected.
type Direction string

const (
	// KindRegex matches a user-provided RE2 regular expression.
	KindRegex Kind = "regex"
	// KindEmail matches email address candidates.
	KindEmail Kind = "email"
	// KindPhone matches phone number candidates.
	KindPhone Kind = "phone"
	// KindPaymentCard matches payment-card candidates that pass Luhn validation.
	KindPaymentCard Kind = "payment_card"
	// KindSecret matches secret or token candidates.
	KindSecret Kind = "secret"

	// DirectionInput inspects request-side text.
	DirectionInput Direction = "input"
	// DirectionOutput inspects response-side text.
	DirectionOutput Direction = "output"
)

// Definition describes one detector that will be compiled into the runtime registry.
type Definition struct {
	Key        string
	Kind       Kind
	Pattern    string
	Categories []string
}

// Input contains text and metadata for detector evaluation.
type Input struct {
	Direction Direction
	Text      string
}

// Finding describes a detector match without retaining the raw matched value.
type Finding struct {
	DetectorKey string
	Category    string
	Start       int
	End         int
	ValueHash   string
}

// Detector evaluates text and returns privacy-safe findings.
type Detector interface {
	Key() string
	Match(Input) []Finding
}

// Registry holds compiled detectors for repeated evaluation.
type Registry struct {
	detectors []Detector
}

// CompileAll compiles all detector definitions into a registry.
func CompileAll(defs []Definition) (Registry, error) {
	compiled := make([]Detector, 0, len(defs))
	for _, def := range defs {
		detector, err := Compile(def)
		if err != nil {
			return Registry{}, err
		}
		compiled = append(compiled, detector)
	}
	return Registry{detectors: compiled}, nil
}

// Match evaluates all detectors in the registry against the input text.
func (r Registry) Match(input Input) []Finding {
	var findings []Finding
	for _, detector := range r.detectors {
		findings = append(findings, detector.Match(input)...)
	}
	return findings
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
