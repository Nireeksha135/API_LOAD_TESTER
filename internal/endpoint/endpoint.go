// Package endpoint derives stable, low-cardinality endpoint labels from the
// URLs a load test is configured with, so results can be grouped per endpoint
// instead of per concrete URL.
package endpoint

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// IDPlaceholder replaces path segments that identify a single resource.
const IDPlaceholder = "{id}"

var (
	templateToken = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	uuidSegment   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hexSegment    = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
	digitsSegment = regexp.MustCompile(`^[0-9]+$`)
)

// Normalize returns the endpoint key for rawURL: the path only, with the query
// string and fragment removed and identifier-like segments collapsed to {id}.
//
// Segments are collapsed when they contain a {{counter}} or {{random}} template
// token, are all digits, are UUIDs, or are long hexadecimal strings (hashes,
// object IDs). Everything else is kept as written, so /login and /checkout stay
// distinct while /orders/17 and /orders/{{counter}} both become /orders/{id}.
func Normalize(rawURL string) string {
	return pathOf(rawURL)
}

// Labels returns one endpoint label per URL, in the same order as urls.
// Labels are path-only unless the URLs span more than one host, in which case
// the host is kept as a prefix so identical paths on different hosts stay apart.
func Labels(urls []string) []string {
	hosts := make(map[string]struct{}, len(urls))
	parsedHosts := make([]string, len(urls))
	for i, raw := range urls {
		host := hostOf(raw)
		parsedHosts[i] = host
		hosts[host] = struct{}{}
	}
	withHost := len(hosts) > 1

	labels := make([]string, len(urls))
	for i, raw := range urls {
		label := pathOf(raw)
		if withHost && parsedHosts[i] != "" {
			label = parsedHosts[i] + label
		}
		labels[i] = label
	}
	return labels
}

// Distinct returns the sorted set of unique labels.
func Distinct(labels []string) []string {
	seen := make(map[string]struct{}, len(labels))
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

func hostOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Host)
}

func pathOf(rawURL string) string {
	path := rawURL
	if parsed, err := url.Parse(rawURL); err == nil && (parsed.Scheme != "" || parsed.Host != "") {
		path = parsed.EscapedPath()
	} else {
		// Unparseable or scheme-less input: strip the query/fragment by hand.
		if i := strings.IndexAny(path, "?#"); i >= 0 {
			path = path[:i]
		}
	}

	segments := strings.Split(path, "/")
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		// Unescape per segment so template tokens such as {{counter}}, which
		// url.URL percent-encodes, are recognised.
		if unescaped, err := url.PathUnescape(segment); err == nil {
			segment = unescaped
		}
		kept = append(kept, normalizeSegment(segment))
	}
	return "/" + strings.Join(kept, "/")
}

func normalizeSegment(segment string) string {
	if templateToken.MatchString(segment) {
		return templateToken.ReplaceAllString(segment, IDPlaceholder)
	}
	if digitsSegment.MatchString(segment) || uuidSegment.MatchString(segment) || hexSegment.MatchString(segment) {
		return IDPlaceholder
	}
	return segment
}
