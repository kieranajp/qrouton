package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// linearAPI is overridable in tests.
var linearAPI = linearAPIDefault

var linearIdentifierPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-[1-9][0-9]*$`)

type linear struct{}

func (linear) Name() string  { return linearProvider }
func (linear) Label() string { return linearLabel }

func (linear) Hosts() []string { return []string{linearHost} }

func (linear) Parse(u *url.URL) (Reference, error) {
	if len(u.String()) > linearMaxReferenceBytes || u.User != nil || u.Port() != "" {
		return Reference{}, ErrNotLinearIssue
	}
	id, err := linearIdentifier(u)
	if err != nil {
		return Reference{}, err
	}
	return linearReference(id), nil
}

// ParseBare accepts the identifier Linear Desktop hands over, which arrives with
// no URL around it.
func (linear) ParseBare(raw string) (Reference, bool) {
	id, ok := normalizeLinearIdentifier(raw)
	if !ok {
		return Reference{}, false
	}
	return linearReference(id), true
}

func linearReference(id string) Reference {
	return Reference{provider: linear{}, ID: id, Canonical: linearCanonicalPrefix + id, Key: id}
}

func (l linear) Fetch(ctx context.Context, client *http.Client, ref Reference) (Ticket, error) {
	token := strings.TrimSpace(os.Getenv(linearTokenEnvVar))
	if token == "" {
		return Ticket{}, ErrNoLinearToken
	}
	payload, _ := json.Marshal(map[string]any{
		linearQueryKey: linearIssueQuery,
		linearVarsKey:  map[string]string{linearIDVar: ref.ID},
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
		return Ticket{}, providerError(l.Name(), err)
	}
	if len(response.Errors) > 0 {
		return Ticket{}, providerError(l.Name(), errors.New(response.Errors[0].Message))
	}
	if response.Data.Issue.Title == "" {
		return Ticket{}, notFound(l.Name())
	}
	return Ticket{Title: response.Data.Issue.Title, Body: response.Data.Issue.Description}, nil
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
