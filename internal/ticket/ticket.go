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
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme != httpsScheme {
		return nil, ErrUnsupportedProvider
	}
	switch strings.ToLower(u.Hostname()) {
	case linearHost:
		parts := pathSegments(u)
		if len(parts) < linearMinSegments || parts[1] != linearIssueSegment || parts[linearIssueIndex] == "" {
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

// pathSegments splits a ticket URL's path, which both providers address
// positionally. ParseURL has already validated the segment count.
func pathSegments(u *url.URL) []string {
	return strings.Split(strings.Trim(u.Path, pathSeparator), pathSeparator)
}

func fetchLinear(ctx context.Context, client *http.Client, u *url.URL) (Ticket, error) {
	parts := pathSegments(u)
	token := strings.TrimSpace(os.Getenv(linearTokenEnvVar))
	if token == "" {
		return Ticket{}, ErrNoLinearToken
	}
	payload, _ := json.Marshal(map[string]any{
		linearQueryKey: linearIssueQuery,
		linearVarsKey:  map[string]string{linearIDVar: parts[linearIssueIndex]},
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
