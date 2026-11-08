package httpx

import (
	"mime"
	"net/http"
	"strings"
)

// Negotiate selects the most acceptable offered media type, preferring offer order
// on ties. The first offer is the fallback for absent or unsupported Accept values.
// Callers provide valid media types and render the returned format themselves.
// Malformed ranges are ignored; specific exclusions override broader wildcards.
func Negotiate(r *http.Request, offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	accept := strings.Join(r.Header.Values("Accept"), ",")
	if strings.Trim(accept, " \t,") == "" {
		return offers[0]
	}
	ranges := splitMediaRanges(accept)
	best, bestQuality := offers[0], 0
	for _, offer := range offers {
		mediaType, params, err := mime.ParseMediaType(offer)
		if err != nil {
			continue
		}
		quality := mediaQuality(ranges, mediaType, params)
		if quality > bestQuality {
			best, bestQuality = offer, quality
		}
	}
	return best
}

func mediaQuality(ranges []string, offered string, offeredParams map[string]string) int {
	quality, specificity := 0, -1
	major, _, _ := strings.Cut(offered, "/")
	for _, item := range ranges {
		mediaType, params, err := mime.ParseMediaType(item)
		if err != nil {
			continue
		}
		q := 1000
		if value, ok := params["q"]; ok {
			var valid bool
			q, valid = parseQuality(value)
			if !valid {
				continue
			}
			delete(params, "q")
		}
		rank := -1
		switch {
		case mediaType == offered:
			rank = 2
		case mediaType == major+"/*":
			rank = 1
		case mediaType == "*/*":
			rank = 0
		}
		for key, value := range params {
			offeredValue, ok := offeredParams[key]
			if !ok || (value != offeredValue && !(key == "charset" && strings.EqualFold(value, offeredValue))) {
				rank = -1
				break
			}
		}
		if rank < 0 {
			continue
		}
		// Type specificity takes precedence over the number of matching parameters.
		rank = rank*(len(offeredParams)+1) + len(params)
		if rank > specificity || (rank == specificity && q > quality) {
			quality, specificity = q, rank
		}
	}
	return quality
}

// Quality values have at most three fractional digits and range from zero to one.
func parseQuality(value string) (int, bool) {
	whole, fraction, _ := strings.Cut(value, ".")
	if (whole != "0" && whole != "1") || len(fraction) > 3 {
		return 0, false
	}
	q := 0
	for _, digit := range fraction {
		if digit < '0' || digit > '9' || (whole == "1" && digit != '0') {
			return 0, false
		}
		q = q*10 + int(digit-'0')
	}
	for i := len(fraction); i < 3; i++ {
		q *= 10
	}
	if whole == "1" {
		q = 1000
	}
	return q, true
}

// Commas inside quoted parameter values do not separate media ranges.
func splitMediaRanges(value string) []string {
	var ranges []string
	start, quoted, escaped := 0, false, false
	for i, char := range value {
		switch {
		case escaped:
			escaped = false
		case quoted && char == '\\':
			escaped = true
		case char == '"':
			quoted = !quoted
		case char == ',' && !quoted:
			ranges = append(ranges, value[start:i])
			start = i + 1
		}
	}
	return append(ranges, value[start:])
}
