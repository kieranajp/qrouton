package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// Overridable in tests.
var (
	linearAPI = linearAPIDefault
	asanaAPI  = asanaAPIDefault
)

// Provider names, used to prefix errors the user sees.
const (
	linearProvider = "linear"
	asanaProvider  = "asana"
)

var linearIdentifierPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-[1-9][0-9]*$`)

type Ticket struct {
	Title string
	Body  string
}

// Fetch resolves a Linear or Asana browser URL and loads the ticket metadata.
func Fetch(ctx context.Context, client *http.Client, rawURL string) (Ticket, error) {
	u, err := ParseURL(rawURL)
	if err != nil {
		return Ticket{}, err
	}
	switch strings.ToLower(u.Hostname()) {
	case linearHost:
		return fetchLinear(ctx, client, u)
	default:
		return fetchAsana(ctx, client, u)
	}
}

// ParseURL validates the ticket providers and browser URL shapes accepted by qrouton.
func ParseURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme != httpsScheme {
		return nil, ErrUnsupportedProvider
	}
	switch strings.ToLower(u.Hostname()) {
	case linearHost:
		if len(trimmed) > linearMaxReferenceBytes || u.User != nil || u.Port() != "" {
			return nil, ErrNotLinearIssue
		}
		if _, err := linearIdentifier(u); err != nil {
			return nil, ErrNotLinearIssue
		}
	case asanaHost:
		parts := pathSegments(u)
		if len(parts) < asanaMinSegments || parts[0] != asanaRootSegment || parts[len(parts)-1] == "" {
			return nil, ErrNotAsanaTask
		}
	default:
		return nil, ErrUnsupportedProvider
	}
	return u, nil
}

// CanonicalLinearURL validates a Linear identifier or issue URL and returns the
// workspace-free URL persisted by external ingress.
func CanonicalLinearURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > linearMaxReferenceBytes {
		return "", ErrInvalidLinearReference
	}
	if id, ok := normalizeLinearIdentifier(trimmed); ok {
		return linearCanonicalPrefix + id, nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme != httpsScheme || strings.ToLower(u.Hostname()) != linearHost ||
		u.User != nil || u.Port() != "" {
		return "", ErrInvalidLinearReference
	}
	id, err := linearIdentifier(u)
	if err != nil {
		return "", ErrInvalidLinearReference
	}
	return linearCanonicalPrefix + id, nil
}

func pathSegments(u *url.URL) []string {
	return strings.Split(strings.Trim(u.Path, pathSeparator), pathSeparator)
}

func linearIdentifier(u *url.URL) (string, error) {
	escaped := u.EscapedPath()
	if !strings.HasPrefix(escaped, pathSeparator) {
		return "", ErrNotLinearIssue
	}
	parts := strings.Split(strings.TrimPrefix(escaped, pathSeparator), pathSeparator)
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", ErrNotLinearIssue
		}
	}
	index := -1
	switch {
	case len(parts) >= linearShortMinSegments && parts[0] == linearIssueSegment:
		index = linearShortIDIndex
	case len(parts) >= linearScopedMinSegments && parts[1] == linearIssueSegment && parts[0] != "":
		index = linearScopedIDIndex
	default:
		return "", ErrNotLinearIssue
	}
	if id, ok := normalizeLinearIdentifier(parts[index]); ok {
		return id, nil
	}
	return "", ErrNotLinearIssue
}

func normalizeLinearIdentifier(raw string) (string, bool) {
	if len(raw) == 0 || len(raw) > linearMaxIdentifierBytes || !linearIdentifierPattern.MatchString(raw) {
		return "", false
	}
	return strings.ToUpper(raw), true
}

func fetchLinear(ctx context.Context, client *http.Client, u *url.URL) (Ticket, error) {
	id, err := linearIdentifier(u)
	if err != nil {
		return Ticket{}, err
	}
	token := strings.TrimSpace(os.Getenv(linearTokenEnvVar))
	if token == "" {
		return Ticket{}, ErrNoLinearToken
	}
	payload, _ := json.Marshal(map[string]any{
		linearQueryKey: linearIssueQuery,
		linearVarsKey:  map[string]string{linearIDVar: id},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearAPI, bytes.NewReader(payload))
	if err != nil {
		return Ticket{}, err
	}
	req.Header.Set(authorizationHeader, token)
	req.Header.Set(contentTypeHeader, contentTypeJSON)
	var response struct {
		Data struct {
			Issue struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			} `json:"issue"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := doJSON(client, req, &response); err != nil {
		return Ticket{}, providerError(linearProvider, err)
	}
	if len(response.Errors) > 0 {
		return Ticket{}, providerError(linearProvider, errors.New(response.Errors[0].Message))
	}
	if response.Data.Issue.Title == "" {
		return Ticket{}, notFound(linearProvider)
	}
	return Ticket{Title: response.Data.Issue.Title, Body: response.Data.Issue.Description}, nil
}

func fetchAsana(ctx context.Context, client *http.Client, u *url.URL) (Ticket, error) {
	parts := pathSegments(u)
	token := strings.TrimSpace(os.Getenv(asanaTokenEnvVar))
	if token == "" {
		return Ticket{}, ErrNoAsanaToken
	}
	endpoint := asanaAPI + asanaTasksPath + url.PathEscape(parts[len(parts)-1]) + asanaTaskFields
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Ticket{}, err
	}
	req.Header.Set(authorizationHeader, bearerPrefix+token)
	var response struct {
		Data struct {
			Name  string `json:"name"`
			Notes string `json:"notes"`
		} `json:"data"`
	}
	if err := doJSON(client, req, &response); err != nil {
		return Ticket{}, providerError(asanaProvider, err)
	}
	if response.Data.Name == "" {
		return Ticket{}, notFound(asanaProvider)
	}
	return Ticket{Title: response.Data.Name, Body: response.Data.Notes}, nil
}

func doJSON(client *http.Client, req *http.Request, dst any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}
