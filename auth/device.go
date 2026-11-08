package auth

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Device metadata is descriptive client input, not proof of device identity.
func normalizeUserAgent(value string) string {
	var result strings.Builder
	for _, char := range value {
		if unicode.IsControl(char) {
			continue
		}
		if result.Len()+utf8.RuneLen(char) > 512 {
			break
		}
		result.WriteRune(char)
	}
	return strings.TrimSpace(result.String())
}
