package openai

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentCredentialResolver(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-secret")
	t.Setenv("KIWIGUARD_CREDENTIAL_SECRET_OPENAI", "prefixed-secret")

	resolver := EnvironmentCredentialResolver{}

	got, err := resolver.ResolveCredential("env:OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("ResolveCredential(env) error = %v", err)
	}
	if got != "env-secret" {
		t.Fatalf("ResolveCredential(env) = %q, want env-secret", got)
	}

	got, err = resolver.ResolveCredential("secret/openai")
	if err != nil {
		t.Fatalf("ResolveCredential(prefixed) error = %v", err)
	}
	if got != "prefixed-secret" {
		t.Fatalf("ResolveCredential(prefixed) = %q, want prefixed-secret", got)
	}

	if _, err := resolver.ResolveCredential("env:DOES_NOT_EXIST"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("ResolveCredential(missing) error = %v, want ErrCredentialNotFound", err)
	}
	if _, err := resolver.ResolveCredential("///"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("ResolveCredential(empty normalized ref) error = %v, want ErrCredentialNotFound", err)
	}
}

func TestFileCredentialResolver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openai.key")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	resolver := FileCredentialResolver{}
	got, err := resolver.ResolveCredential("file:" + path)
	if err != nil {
		t.Fatalf("ResolveCredential(file) error = %v", err)
	}
	if got != "file-secret" {
		t.Fatalf("ResolveCredential(file) = %q, want file-secret", got)
	}

	if _, err := resolver.ResolveCredential("env:OPENAI_API_KEY"); !errors.Is(err, ErrCredentialUnsupportedRef) {
		t.Fatalf("ResolveCredential(unsupported) error = %v, want ErrCredentialUnsupportedRef", err)
	}
	if _, err := resolver.ResolveCredential("file:"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("ResolveCredential(empty file) error = %v, want ErrCredentialNotFound", err)
	}
	if _, err := resolver.ResolveCredential("file:/does/not/exist"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("ResolveCredential(missing file) error = %v, want ErrCredentialNotFound", err)
	}
}

func TestChainCredentialResolverUsesFirstSupportedResolver(t *testing.T) {
	resolver := ChainCredentialResolver{
		CredentialResolverFunc(func(string) (string, error) {
			return "", ErrCredentialUnsupportedRef
		}),
		CredentialResolverFunc(func(ref string) (string, error) {
			return "resolved-" + ref, nil
		}),
	}

	got, err := resolver.ResolveCredential("openai")
	if err != nil {
		t.Fatalf("ResolveCredential() error = %v", err)
	}
	if got != "resolved-openai" {
		t.Fatalf("ResolveCredential() = %q, want resolved-openai", got)
	}

	_, err = (ChainCredentialResolver{}).ResolveCredential("openai")
	if !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("ResolveCredential(empty chain) error = %v, want ErrCredentialNotFound", err)
	}

	if DefaultCredentialResolver() == nil {
		t.Fatal("DefaultCredentialResolver() = nil")
	}
}
