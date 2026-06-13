package detection

import "unicode"

// validLuhn reports whether value passes the Luhn checksum after stripping non-digits.
func validLuhn(value string) bool {
	digits := make([]int, 0, len(value))
	for _, r := range value {
		if unicode.IsDigit(r) {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		digit := digits[i]
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}
