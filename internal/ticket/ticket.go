package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	linearAPI = "https://api.linear.app/graphql"
	asanaAPI  = "https://app.asana.com/api/1.0"
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
	case "linear.app":
		return fetchLinear(ctx, client, u)
	default:
		return fetchAsana(ctx, client, u)
	}
}

// ParseURL validates the ticket providers and browser URL shapes accepted by qrouton.
func ParseURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("ticket must be a Linear or Asana URL")
	}
	switch strings.ToLower(u.Hostname()) {
	case "linear.app":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 3 || parts[1] != "issue" || parts[2] == "" {
			return nil, fmt.Errorf("ticket must be a Linear issue URL")
		}
	case "app.asana.com":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 3 || parts[0] != "0" || parts[len(parts)-1] == "" {
			return nil, fmt.Errorf("ticket must be an Asana task URL")
		}
	default:
		return nil, fmt.Errorf("ticket must be a Linear or Asana URL")
	}
	return u, nil
}

func fetchLinear(ctx context.Context, client *http.Client, u *url.URL) (Ticket, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	token := strings.TrimSpace(os.Getenv("LINEAR_API_KEY"))
	if token == "" {
		return Ticket{}, fmt.Errorf("set LINEAR_API_KEY to load ticket details")
	}
	payload, _ := json.Marshal(map[string]any{"query": `query Ticket($id: String!) { issue(id: $id) { title description } }`, "variables": map[string]string{"id": parts[2]}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearAPI, bytes.NewReader(payload))
	if err != nil {
		return Ticket{}, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
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
		return Ticket{}, fmt.Errorf("linear: loading ticket: %w", err)
	}
	if len(response.Errors) > 0 {
		return Ticket{}, fmt.Errorf("linear: loading ticket: %s", response.Errors[0].Message)
	}
	if response.Data.Issue.Title == "" {
		return Ticket{}, fmt.Errorf("linear: ticket not found")
	}
	return Ticket{Title: response.Data.Issue.Title, Body: response.Data.Issue.Description}, nil
}

func fetchAsana(ctx context.Context, client *http.Client, u *url.URL) (Ticket, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	token := strings.TrimSpace(os.Getenv("ASANA_ACCESS_TOKEN"))
	if token == "" {
		return Ticket{}, fmt.Errorf("set ASANA_ACCESS_TOKEN to load ticket details")
	}
	endpoint := asanaAPI + "/tasks/" + url.PathEscape(parts[len(parts)-1]) + "?opt_fields=name,notes"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Ticket{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	var response struct {
		Data struct {
			Name  string `json:"name"`
			Notes string `json:"notes"`
		} `json:"data"`
	}
	if err := doJSON(client, req, &response); err != nil {
		return Ticket{}, fmt.Errorf("asana: loading ticket: %w", err)
	}
	if response.Data.Name == "" {
		return Ticket{}, fmt.Errorf("asana: ticket not found")
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
