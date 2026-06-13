package detection

import "testing"

func BenchmarkRegistryMatch(b *testing.B) {
	registry, err := CompileAll([]Definition{
		{Key: "email", Kind: KindEmail},
		{Key: "phone", Kind: KindPhone},
		{Key: "card", Kind: KindPaymentCard},
		{Key: "secret", Kind: KindSecret},
	})
	if err != nil {
		b.Fatalf("CompileAll() error = %v", err)
	}
	input := Input{Direction: DirectionInput, Text: "alice@example.com +1 415-555-0199 sk-test-abcdefghijklmnopqrstuvwxyz"}
	b.ReportAllocs()
	for b.Loop() {
		_ = registry.Match(input)
	}
}
