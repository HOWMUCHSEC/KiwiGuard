package openai

import (
	"errors"
	"os"
	"strings"
	"unicode"
)

var (
	// ErrCredentialNotFound reports that a credential reference cannot be resolved.
	ErrCredentialNotFound = errors.New("credential not found")
	// ErrCredentialUnsupportedRef reports that a resolver does not support a reference scheme.
	ErrCredentialUnsupportedRef = errors.New("unsupported credential reference")
)

// CredentialResolver resolves secret-backed credential references into runtime-only secrets.
type CredentialResolver interface {
	ResolveCredential(ref string) (string, error)
}

// CredentialResolverFunc adapts a function into a CredentialResolver.
type CredentialResolverFunc func(string) (string, error)

// ResolveCredential resolves a credential reference.
func (f CredentialResolverFunc) ResolveCredential(ref string) (string, error) {
	return f(ref)
}

// EnvironmentCredentialResolver resolves env:NAME references or bare references
// from KIWIGUARD_CREDENTIAL_<NORMALIZED_REFERENCE> environment variables.
type EnvironmentCredentialResolver struct {
	LookupEnv func(string) (string, bool)
}

// ResolveCredential resolves a credential reference from environment variables.
func (r EnvironmentCredentialResolver) ResolveCredential(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ErrCredentialNotFound
	}

	var key string
	switch {
	case strings.HasPrefix(ref, "env:"):
		key = strings.TrimSpace(strings.TrimPrefix(ref, "env:"))
	case strings.Contains(ref, ":"):
		return "", ErrCredentialUnsupportedRef
	default:
		key = credentialEnvKey(ref)
	}
	if key == "" {
		return "", ErrCredentialNotFound
	}

	lookup := r.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	value, ok := lookup(key)
	if !ok || value == "" {
		return "", ErrCredentialNotFound
	}
	return value, nil
}

// FileCredentialResolver resolves file:/absolute/or/relative/path references.
type FileCredentialResolver struct{}

// ResolveCredential resolves a credential reference from a local file.
func (r FileCredentialResolver) ResolveCredential(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, "file:") {
		return "", ErrCredentialUnsupportedRef
	}
	path := strings.TrimSpace(strings.TrimPrefix(ref, "file:"))
	if path == "" {
		return "", ErrCredentialNotFound
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", ErrCredentialNotFound
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", ErrCredentialNotFound
	}
	return value, nil
}

// ChainCredentialResolver tries a sequence of resolvers until one resolves the reference.
type ChainCredentialResolver []CredentialResolver

// ResolveCredential resolves a credential reference using the first matching resolver.
func (r ChainCredentialResolver) ResolveCredential(ref string) (string, error) {
	var lastErr error
	for _, resolver := range r {
		if resolver == nil {
			continue
		}
		value, err := resolver.ResolveCredential(ref)
		if err == nil {
			return value, nil
		}
		lastErr = err
		if !errors.Is(err, ErrCredentialUnsupportedRef) && !errors.Is(err, ErrCredentialNotFound) {
			return "", err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", ErrCredentialNotFound
}

// DefaultCredentialResolver returns the production resolver chain.
func DefaultCredentialResolver() CredentialResolver {
	return ChainCredentialResolver{
		EnvironmentCredentialResolver{},
		FileCredentialResolver{},
	}
}

func credentialEnvKey(ref string) string {
	const prefix = "KIWIGUARD_CREDENTIAL_"

	var builder strings.Builder
	previousUnderscore := true
	for _, r := range ref {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(unicode.ToUpper(r))
			previousUnderscore = false
			continue
		}
		if !previousUnderscore {
			builder.WriteByte('_')
			previousUnderscore = true
		}
	}
	suffix := strings.TrimRight(builder.String(), "_")
	if suffix == "" {
		return ""
	}
	return prefix + suffix
}
