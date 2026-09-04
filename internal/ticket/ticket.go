package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Ticket struct {
	Title string
	Body  string
}

// Reference is one parsed ticket: the provider that owns it, the id that
// provider fetches by, the URL persisted on the manifest, and the fragment a
// slug and branch are seeded from — empty when the provider seeds nothing.
type Reference struct {
	provider  Provider
	ID        string
	Canonical string
	Key       string
}

type Provider interface {
	Name() string
	// Label is the provider as a person writes it, which is how a prompt
	// attributes the request it carries.
	Label() string
	Hosts() []string
	Parse(*url.URL) (Reference, error)
	Fetch(context.Context, *http.Client, Reference) (Ticket, error)
}

// bareParser is implemented only by a provider that accepts a reference with no
// URL around it. Linear Desktop hands over "LIF-2841".
type bareParser interface {
	ParseBare(string) (Reference, bool)
}

var providers = []Provider{linear{}, asana{}, github{}}

// Validate reports whether a pasted link is a ticket some provider owns. A bare
// reference is not one: the field holds URLs.
func Validate(raw string) error {
	_, err := parseURL(raw)
	return err
}

// Canonical is the URL a session persists, and the one dedupe compares. A
// provider with no canonical form of its own answers with the link as given.
func Canonical(raw string) (string, error) {
	ref, err := parse(raw)
	if err != nil {
		return "", err
	}
	return ref.Canonical, nil
}

// Key is the ticket fragment a slug and branch are seeded from, and empty for
// anything no provider claims or seeds.
func Key(raw string) string {
	ref, err := parse(raw)
	if err != nil {
		return ""
	}
	return ref.Key
}

// ProviderLabel names the provider that owns raw, and is empty for anything no
// provider claims.
func ProviderLabel(raw string) string {
	ref, err := parse(raw)
	if err != nil {
		return ""
	}
	return ref.provider.Label()
}

func Fetch(ctx context.Context, client *http.Client, rawURL string) (Ticket, error) {
	ref, err := parseURL(rawURL)
	if err != nil {
		return Ticket{}, err
	}
	return ref.provider.Fetch(ctx, client, ref)
}

// parse accepts everything a provider owns, bare references included.
func parse(raw string) (Reference, error) {
	trimmed := strings.TrimSpace(raw)
	for _, p := range providers {
		bare, ok := p.(bareParser)
		if !ok {
			continue
		}
		if ref, ok := bare.ParseBare(trimmed); ok {
			return ref, nil
		}
	}
	return parseURL(trimmed)
}

func parseURL(raw string) (Reference, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != httpsScheme {
		return Reference{}, ErrUnsupportedProvider
	}
	provider, ok := providerFor(u)
	if !ok {
		return Reference{}, ErrUnsupportedProvider
	}
	return provider.Parse(u)
}

func providerFor(u *url.URL) (Provider, bool) {
	host := strings.ToLower(u.Hostname())
	for _, p := range providers {
		for _, claimed := range p.Hosts() {
			if claimed == host {
				return p, true
			}
		}
	}
	return nil, false
}

func pathSegments(u *url.URL) []string {
	return strings.Split(strings.Trim(u.Path, pathSeparator), pathSeparator)
}

// statusError is a non-2xx answer, carrying the code so a provider can map one
// status of its own while every other status keeps the generic message.
type statusError struct {
	code   int
	status string
}

func (e statusError) Error() string { return fmt.Sprintf(requestFailedFormat, e.status) }

func doJSON(client *http.Client, req *http.Request, dst any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError{code: resp.StatusCode, status: resp.Status}
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf(decodingResponseFormat, err)
	}
	return nil
}
