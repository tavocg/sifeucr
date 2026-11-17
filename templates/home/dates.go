package home

import (
	"fmt"
	"time"
)

var months = map[string][12]string{
	"en": {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
	"es": {"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"},
}

// isoTimestamp formats a Unix timestamp as an RFC 3339 UTC instant for <time>.
// The browser client rewrites the rendered text in the visitor's timezone.
func isoTimestamp(value int64) string {
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

// date formats a Unix timestamp in UTC for the session table. It is the fallback
// content of the <time> element, which the browser client rewrites in the
// visitor's timezone. Spanish puts the day before the abbreviated month
// ("11 feb") and English after it ("Feb 11"). The year is only shown when it
// differs from the current year, and the 12-hour clock is only added when
// withTime is set.
func date(language string, value int64, withTime bool) string {
	if value == 0 {
		return "—"
	}

	t := time.Unix(value, 0).UTC()
	names, ok := months[language]
	if !ok {
		names = months["en"]
	}
	month := names[t.Month()-1]

	var formatted string
	if language == "es" {
		formatted = fmt.Sprintf("%d %s", t.Day(), month)
	} else {
		formatted = fmt.Sprintf("%s %d", month, t.Day())
	}

	if t.Year() != time.Now().UTC().Year() {
		if language == "es" {
			formatted += fmt.Sprintf(" %d", t.Year())
		} else {
			formatted += fmt.Sprintf(", %d", t.Year())
		}
	}
	if !withTime {
		return formatted
	}
	return formatted + ", " + clock(language, t)
}

// clock renders the 12-hour time used in the Last used column.
func clock(language string, t time.Time) string {
	hour := t.Hour() % 12
	if hour == 0 {
		hour = 12
	}

	spanish := language == "es"
	meridiem := "AM"
	if spanish {
		meridiem = "a. m."
	}
	if t.Hour() >= 12 {
		if spanish {
			meridiem = "p. m."
		} else {
			meridiem = "PM"
		}
	}
	return fmt.Sprintf("%d:%02d %s", hour, t.Minute(), meridiem)
}
