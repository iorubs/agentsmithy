// Package urlallow validates URLs against a scheme+host allowlist and
// provides an HTTP client that refuses redirects (so a permitted URL
// can't silently lead to a denied one).
//
// Copied from mcpsmithy/internal/tools/templating.go; will be deduped
// when a shared module is extracted.
package urlallow

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// NoRedirectClient is an HTTP client that surfaces redirects as
// errors rather than following them.
var NoRedirectClient = &http.Client{
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Parse takes a slice of full URLs (e.g. "https://api.github.com")
// and returns a set of scheme://host strings to compare against. An
// empty or nil input returns nil.
func Parse(urls []string) map[string]bool {
	if len(urls) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(urls))
	for _, raw := range urls {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		allowed[strings.ToLower(u.Scheme)+"://"+strings.ToLower(u.Host)] = true
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

// Check returns nil when rawURL's scheme+host is in allowed. A nil
// allowed map means "no restrictions" — callers use a non-nil map
// (even empty) to deny all.
func Check(rawURL string, allowed map[string]bool) error {
	if allowed == nil {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	key := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	if !allowed[key] {
		return fmt.Errorf("URL %q is not in the allowed list", rawURL)
	}
	return nil
}
