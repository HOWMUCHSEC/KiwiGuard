package traffic

import "testing"

func TestHashBodyReturnsSHA256HexDigest(t *testing.T) {
	got := HashBody([]byte("kiwiguard"))
	want := "ea683cd199162a0bbc4986acf1cf4d3300e1d1134b5a4fd6bf0edd3a2f7ffd2a"
	if got != want {
		t.Fatalf("HashBody() = %q, want %q", got, want)
	}
}
