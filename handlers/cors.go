package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// RouteOptions overrides the handler policy. Nil inherits; an empty non-nil
// slice disables CORS. Origins are exact HTTP(S) origins, without paths.
type RouteOptions struct{ CORSOrigins []string }

type routePolicy struct {
	method  string
	origins []string
}

func validateOrigins(origins []string) error {
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || strings.ContainsAny(origin, "?#") {
			return fmt.Errorf("invalid origin %q: use an exact HTTP(S) origin without a path", origin)
		}
	}
	return nil
}

func (h *Handler) corsHeaders(w http.ResponseWriter, r *http.Request, origins []string) {
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" && (h.dev || slices.Contains(origins, origin)) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
}

// options derives methods and policies from the same mux used for dispatch.
// It never invokes authentication or an endpoint during a preflight.
func (h *Handler) options(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	_, explicit := h.mux.Handler(r)
	if policy, ok := h.routePolicies[explicit]; ok && (policy.method == http.MethodOptions || policy.method == "") {
		return false
	}
	allowed := make(map[string]routePolicy)
	for method := range h.methods {
		if method == "" || method == http.MethodOptions {
			continue
		}
		probe := new(http.Request)
		*probe = *r
		probe.Method = method
		_, pattern := h.mux.Handler(probe)
		if policy, ok := h.routePolicies[pattern]; ok {
			allowed[method] = policy
		}
	}
	if len(allowed) == 0 {
		return false
	}
	methods := []string{http.MethodOptions}
	for method := range allowed {
		methods = append(methods, method)
	}
	slices.Sort(methods)
	w.Header().Set("Allow", strings.Join(methods, ", "))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
	origin := r.Header.Get("Origin")
	requested := r.Header.Get("Access-Control-Request-Method")
	if origin != "" && requested != "" {
		policy, ok := allowed[requested]
		permitted := ok && (h.dev || slices.Contains(policy.origins, origin))
		for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
			switch strings.ToLower(strings.TrimSpace(header)) {
			case "", "authorization", "content-type", "accept", "accept-language":
			default:
				permitted = false
			}
		}
		if !permitted {
			h.writeStatus(
				w,
				r,
				http.StatusForbidden,
				"err.cors_denied",
				"CORS preflight denied",
			)
			return true
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", requested)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Accept-Language")
	}
	w.WriteHeader(http.StatusNoContent)
	return true
}
