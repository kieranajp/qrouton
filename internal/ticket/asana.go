package ticket

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// asanaAPI is overridable in tests.
var asanaAPI = asanaAPIDefault

type asana struct{}

func (asana) Name() string { return asanaProvider }

func (asana) Hosts() []string { return []string{asanaHost} }

// Parse seeds nothing: an Asana URL carries no fragment a branch would want, and
// the task gid is not one either. Asana has no canonical form, so the link
// re-serialised is the closest thing dedupe can compare.
func (asana) Parse(u *url.URL) (Reference, error) {
	parts := pathSegments(u)
	if len(parts) < asanaMinSegments || parts[0] != asanaRootSegment || parts[len(parts)-1] == "" {
		return Reference{}, ErrNotAsanaTask
	}
	return Reference{provider: asana{}, ID: parts[len(parts)-1], Canonical: u.String()}, nil
}

func (a asana) Fetch(ctx context.Context, client *http.Client, ref Reference) (Ticket, error) {
	token := strings.TrimSpace(os.Getenv(asanaTokenEnvVar))
	if token == "" {
		return Ticket{}, ErrNoAsanaToken
	}
	endpoint := asanaAPI + asanaTasksPath + url.PathEscape(ref.ID) + asanaTaskFields
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
		return Ticket{}, providerError(a.Name(), err)
	}
	if response.Data.Name == "" {
		return Ticket{}, notFound(a.Name())
	}
	return Ticket{Title: response.Data.Name, Body: response.Data.Notes}, nil
}
