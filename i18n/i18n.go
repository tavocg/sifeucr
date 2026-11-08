// Package i18n provides isolated copies of the application's locales.
package i18n

import (
	"maps"

	"github.com/tavocg/go-i18n"
)

// Locales returns fresh maps so consumers cannot mutate the application's messages.
func Locales() map[string]i18n.Locale {
	return map[string]i18n.Locale{
		"en": maps.Clone(en),
		"es": maps.Clone(es),
	}
}
