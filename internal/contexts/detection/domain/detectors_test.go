package detection

import "testing"

func TestRegexDetectorMatchesConfiguredPattern(t *testing.T) {
	detector, err := Compile(Definition{
		Key:        "custom-email",
		Kind:       KindRegex,
		Pattern:    `(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`,
		Categories: []string{"pii.email"},
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	findings := detector.Match(Input{Direction: DirectionInput, Text: "contact alice@example.com"})
	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1", len(findings))
	}
	if findings[0].DetectorKey != "custom-email" {
		t.Fatalf("DetectorKey = %q, want custom-email", findings[0].DetectorKey)
	}
	if findings[0].Category != "pii.email" {
		t.Fatalf("Category = %q, want pii.email", findings[0].Category)
	}
	if findings[0].Start < 0 || findings[0].End <= findings[0].Start {
		t.Fatalf("invalid span: %+v", findings[0])
	}
	if findings[0].ValueHash == "" {
		t.Fatal("ValueHash is empty")
	}
	if detector.Key() != "custom-email" {
		t.Fatalf("Key() = %q, want custom-email", detector.Key())
	}
}

func TestCompileRejectsInvalidRegex(t *testing.T) {
	_, err := Compile(Definition{Key: "bad", Kind: KindRegex, Pattern: "["})
	if err == nil {
		t.Fatal("Compile() error = nil, want error")
	}
}

func TestBuiltInPIIDetectors(t *testing.T) {
	registry, err := CompileAll([]Definition{
		{Key: "email", Kind: KindEmail},
		{Key: "phone", Kind: KindPhone},
		{Key: "card", Kind: KindPaymentCard},
		{Key: "secret", Kind: KindSecret},
	})
	if err != nil {
		t.Fatalf("CompileAll() error = %v", err)
	}

	input := Input{
		Direction: DirectionInput,
		Text:      "email bob@example.com phone +1 415-555-0199 card 4242 4242 4242 4242 token sk-test-abcdefghijklmnopqrstuvwxyz",
	}
	findings := registry.Match(input)
	wantCategories := map[string]bool{
		"pii.email":        false,
		"pii.phone":        false,
		"pii.payment_card": false,
		"secret.token":     false,
	}
	for _, finding := range findings {
		if _, ok := wantCategories[finding.Category]; ok {
			wantCategories[finding.Category] = true
		}
	}
	for category, found := range wantCategories {
		if !found {
			t.Fatalf("missing category %s in findings %+v", category, findings)
		}
	}
}

func TestPaymentCardRequiresLuhn(t *testing.T) {
	detector, err := Compile(Definition{Key: "card", Kind: KindPaymentCard})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	findings := detector.Match(Input{Direction: DirectionInput, Text: "not a card 4242 4242 4242 4241"})
	if len(findings) != 0 {
		t.Fatalf("len(findings) = %d, want 0: %+v", len(findings), findings)
	}
}

func TestCompileRejectsMissingKeyUnknownKindAndEmptyPattern(t *testing.T) {
	tests := []struct {
		name string
		def  Definition
	}{
		{name: "missing key", def: Definition{Kind: KindRegex, Pattern: `\d+`}},
		{name: "unknown kind", def: Definition{Key: "unknown", Kind: Kind("unknown")}},
		{name: "empty custom pattern", def: Definition{Key: "empty", Kind: KindRegex}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Compile(tt.def); err == nil {
				t.Fatal("Compile() error = nil, want error")
			}
		})
	}
}

func TestDetectorUsesFallbackCategoryWhenCategoriesAreEmpty(t *testing.T) {
	detector, err := Compile(Definition{
		Key:        "fallback",
		Kind:       KindRegex,
		Pattern:    `secret-\d+`,
		Categories: []string{"", ""},
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	findings := detector.Match(Input{Text: "secret-123"})
	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1", len(findings))
	}
	if findings[0].Category != defaultRegexCategory {
		t.Fatalf("Category = %q, want %q", findings[0].Category, defaultRegexCategory)
	}
}
