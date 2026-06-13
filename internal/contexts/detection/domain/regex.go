package detection

import (
	"errors"
	"fmt"
	"regexp"
)

const defaultRegexCategory = "custom.regex"

type regexDetector struct {
	key        string
	re         *regexp.Regexp
	categories []string
	validator  func(string) bool
}

// Compile compiles a detector definition.
func Compile(def Definition) (Detector, error) {
	if def.Key == "" {
		return nil, errors.New("compile detector: key is required")
	}

	pattern, categories, validator, err := normalizeDefinition(def)
	if err != nil {
		return nil, fmt.Errorf("compile detector %s: %w", def.Key, err)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile detector %s: %w", def.Key, err)
	}

	return regexDetector{
		key:        def.Key,
		re:         re,
		categories: categories,
		validator:  validator,
	}, nil
}

func (d regexDetector) Key() string {
	return d.key
}

func (d regexDetector) Match(input Input) []Finding {
	if input.Text == "" {
		return nil
	}

	indexes := d.re.FindAllStringIndex(input.Text, -1)
	if len(indexes) == 0 {
		return nil
	}

	findings := make([]Finding, 0, len(indexes)*len(d.categories))
	for _, index := range indexes {
		value := input.Text[index[0]:index[1]]
		if d.validator != nil && !d.validator(value) {
			continue
		}
		valueHash := hashValue(value)
		for _, category := range d.categories {
			findings = append(findings, Finding{
				DetectorKey: d.key,
				Category:    category,
				Start:       index[0],
				End:         index[1],
				ValueHash:   valueHash,
			})
		}
	}
	return findings
}

func normalizeDefinition(def Definition) (string, []string, func(string) bool, error) {
	switch def.Kind {
	case KindRegex:
		return customPattern(def.Pattern, def.Categories, defaultRegexCategory)
	case KindEmail:
		return builtInPattern(def.Pattern, def.Categories, emailPattern, "pii.email", nil)
	case KindPhone:
		return builtInPattern(def.Pattern, def.Categories, phonePattern, "pii.phone", nil)
	case KindPaymentCard:
		return builtInPattern(def.Pattern, def.Categories, paymentCardPattern, "pii.payment_card", validLuhn)
	case KindSecret:
		return builtInPattern(def.Pattern, def.Categories, secretPattern, "secret.token", nil)
	default:
		return "", nil, nil, fmt.Errorf("unknown detector kind %q", def.Kind)
	}
}

func customPattern(pattern string, categories []string, fallbackCategory string) (string, []string, func(string) bool, error) {
	if pattern == "" {
		return "", nil, nil, errors.New("pattern is required")
	}
	return pattern, normalizeCategories(categories, fallbackCategory), nil, nil
}

func builtInPattern(pattern string, categories []string, fallbackPattern string, fallbackCategory string, validator func(string) bool) (string, []string, func(string) bool, error) {
	if pattern == "" {
		pattern = fallbackPattern
	}
	return pattern, normalizeCategories(categories, fallbackCategory), validator, nil
}

func normalizeCategories(categories []string, fallback string) []string {
	if len(categories) == 0 {
		return []string{fallback}
	}

	normalized := make([]string, 0, len(categories))
	for _, category := range categories {
		if category == "" {
			continue
		}
		normalized = append(normalized, category)
	}
	if len(normalized) == 0 {
		return []string{fallback}
	}
	return normalized
}
