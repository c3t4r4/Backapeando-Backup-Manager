// Package cpf validates Brazilian CPF (Cadastro de Pessoas Físicas) numbers.
package cpf

import (
	"fmt"
	"strings"
)

// Validate normalizes raw (stripping any non-digit characters, e.g. "." and
// "-") and validates it as a CPF, including the official check-digit
// algorithm used by the Receita Federal. It returns the normalized 11-digit
// form on success.
func Validate(raw string) (string, error) {
	digits := onlyDigits(raw)
	if len(digits) != 11 {
		return "", fmt.Errorf("cpf must have 11 digits, got %d", len(digits))
	}
	if allSameDigit(digits) {
		return "", fmt.Errorf("cpf with all repeated digits is invalid")
	}
	if !hasValidCheckDigits(digits) {
		return "", fmt.Errorf("cpf check digits do not match")
	}
	return digits, nil
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func allSameDigit(digits string) bool {
	for i := 1; i < len(digits); i++ {
		if digits[i] != digits[0] {
			return false
		}
	}
	return true
}

func hasValidCheckDigits(digits string) bool {
	d := make([]int, len(digits))
	for i, r := range digits {
		d[i] = int(r - '0')
	}
	return d[9] == checkDigit(d[:9], 10) && d[10] == checkDigit(d[:10], 11)
}

// checkDigit computes one CPF verification digit from base digits, with
// descending weights starting at startWeight (10 for the first check digit,
// computed from the first 9 digits; 11 for the second, computed from the
// first 10 digits including the first check digit).
func checkDigit(base []int, startWeight int) int {
	sum := 0
	weight := startWeight
	for _, v := range base {
		sum += v * weight
		weight--
	}
	remainder := sum % 11
	if remainder < 2 {
		return 0
	}
	return 11 - remainder
}

// GenerateValidForTests builds a check-digit-valid CPF from a 9-digit base,
// for tests that need a fresh, non-colliding CPF (e.g. integration tests
// seeding admin_users, whose cpf column is UNIQUE). It exists so tests don't
// have to duplicate the check-digit algorithm above; it is not meant for
// production use.
func GenerateValidForTests(base9 string) (string, error) {
	digits := onlyDigits(base9)
	if len(digits) != 9 {
		return "", fmt.Errorf("base must have 9 digits, got %d", len(digits))
	}
	d := make([]int, 9)
	for i, r := range digits {
		d[i] = int(r - '0')
	}
	if allSameDigit(digits) {
		d[0] = (d[0] + 1) % 10
		digits = fmt.Sprintf("%d%s", d[0], digits[1:])
	}
	c1 := checkDigit(d, 10)
	c2 := checkDigit(append(append([]int{}, d...), c1), 11)
	return fmt.Sprintf("%s%d%d", digits, c1, c2), nil
}
