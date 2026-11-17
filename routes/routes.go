// Package routes is the shared catalogue for application registration and links.
package routes

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"unicode"
)

type ID uint8

const (
	Root ID = iota
	Login
	LoginPage
	Verify
	Refresh
	Account
	DeleteAccount
	Sessions
	RevokeSessions
	RevokeSession
	ChangeEmail
	ConfirmEmail
	Static
	APIDocs
	count
)

type Definition struct{ Method, Path string }

var definitions = [count]Definition{
	Root:           {"GET", "/"},
	Login:          {"POST", "/auth/login"},
	LoginPage:      {"GET", "/login"},
	Verify:         {"POST", "/auth/verify"},
	Refresh:        {"POST", "/auth/refresh"},
	Account:        {"GET", "/account"},
	DeleteAccount:  {"DELETE", "/account"},
	Sessions:       {"GET", "/account/sessions"},
	RevokeSessions: {"DELETE", "/account/sessions"},
	RevokeSession:  {"DELETE", "/account/sessions/{id}"},
	ChangeEmail:    {"PATCH", "/account/email/change"},
	ConfirmEmail:   {"PATCH", "/account/email/change/confirm"},
	Static:         {"GET", "/s/"},
	APIDocs:        {"GET", "/s/api/{$}"},
}

func (id ID) Definition() (Definition, error) {
	if id >= count {
		return Definition{}, fmt.Errorf("unknown route ID %d", id)
	}
	d := definitions[id]
	if d.Method == "" || d.Path == "" {
		return Definition{}, fmt.Errorf("route %d has no definition", id)
	}
	return d, nil
}

func All() []ID {
	ids := make([]ID, count)
	for i := range ids {
		ids[i] = ID(i)
	}
	return ids
}

// URL builds a link using this instance’s application prefix. Wildcards require
// one value each; invalid IDs or missing values return an error, including in templ.
func (l Links) URL(id ID, values ...string) (string, error) {
	d, err := id.Definition()
	if err != nil {
		return "", err
	}
	path := strings.TrimSuffix(d.Path, "{$}")
	for strings.Contains(path, "{") {
		start := strings.IndexByte(path, '{')
		end := strings.IndexByte(path[start:], '}') + start
		if len(values) == 0 || values[0] == "" || values[0] == "." || values[0] == ".." || strings.ContainsAny(values[0], "/\\") {
			return "", fmt.Errorf("route %d requires a nonempty path segment", id)
		}
		path = path[:start] + url.PathEscape(values[0]) + path[end+1:]
		values = values[1:]
	}
	if len(values) != 0 {
		return "", fmt.Errorf("too many parameters for route %d", id)
	}
	return l.Mount(path), nil
}

// Links holds the validated mount prefix for one handler. Its zero value mounts
// at /. Values can be copied into templates and used concurrently without mutation.
type Links struct{ prefix string }

func NewLinks(prefix string) (Links, error) {
	normalized, err := normalizeRootPrefix(prefix)
	if err != nil {
		return Links{}, err
	}
	return Links{prefix: normalized}, nil
}

// Mount prefixes an application-relative route pattern for registration.
// Use URL for named links and StaticURL for embedded assets.
func (l Links) Mount(route string) string {
	route = strings.TrimSpace(route)
	if route == "" || route == "/" {
		if l.prefix == "" {
			return "/"
		}
		return l.prefix
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	return l.prefix + route
}

// StaticURL builds a URL for a relative filename inside the embedded static tree.
func (l Links) StaticURL(name string) (string, error) {
	if !fs.ValidPath(name) || name == "." || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid static asset path %q", name)
	}
	segments := strings.Split(name, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	base, err := l.URL(Static)
	if err != nil {
		return "", err
	}
	return base + strings.Join(segments, "/"), nil
}

func normalizeRootPrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return "", nil
	}

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "", nil
	}
	if path.Clean(prefix) != prefix || strings.ContainsAny(prefix, "{}%?#\\") {
		return "", errors.New("root prefix must be a clean literal URL path without wildcards, escapes, query strings or fragments")
	}
	if strings.ContainsFunc(prefix, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return "", errors.New("root prefix cannot contain whitespace or control characters")
	}
	return prefix, nil
}
