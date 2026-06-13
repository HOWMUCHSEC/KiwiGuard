package detection

// Built-in detector regexes for common PII and secret-looking content.
const (
	// emailPattern matches common email addresses.
	emailPattern = `(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`

	// phonePattern matches common North American-style phone numbers with optional country code.
	phonePattern = `(?i)\b(?:\+?\d{1,3}[\s.-]?)?(?:\(?\d{3}\)?[\s.-]?)\d{3}[\s.-]?\d{4}\b`

	// paymentCardPattern matches potential payment card numbers before Luhn validation.
	paymentCardPattern = `\b(?:\d[ -]?){13,19}\b`

	// secretPattern matches common token and secret key prefixes.
	secretPattern = `(?i)\b(?:sk|pk|api|token|secret)[-_][a-z0-9][a-z0-9_-]{16,}\b`
)
