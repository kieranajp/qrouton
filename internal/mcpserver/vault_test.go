package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type vaultHost struct {
	fakeHost
	result  vault.Result
	excerpt vault.Excerpt
	queries []vault.Query
	reads   []vault.ReadRequest
}

func (h *vaultHost) SearchVault(_ context.Context, q vault.Query) (vault.Result, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.queries = append(h.queries, q)
	return h.result, nil
}

func (h *vaultHost) ReadVault(_ context.Context, req vault.ReadRequest) (vault.Excerpt, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reads = append(h.reads, req)
	return h.excerpt, nil
}

func connectVault(t *testing.T, host workbench.WindowHost) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(t.TempDir(), testEditor, host, session.ModeRPI)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close(); _ = ss.Close() })
	return cs
}

func callText(t *testing.T, cs *mcp.ClientSession, name string, args any) (string, map[string]any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil || res.IsError {
		t.Fatalf("%s = %+v, %v", name, res, err)
	}
	return res.Content[0].(*mcp.TextContent).Text, structuredOutput(t, res.StructuredContent)
}

func TestThoughtsToolsNeedOnlyAHostWithAnIndex(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	names := []string{"search_thoughts", "read_thoughts", "open_thoughts"}
	for _, tc := range []struct {
		name string
		host workbench.WindowHost
		want bool
	}{
		{"index host", &vaultHost{}, true},
		{"host without an index", &fakeHost{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			advertised := advertisedTools(t, newMCPServer(t.TempDir(), testEditor, tc.host, session.ModeRPI))
			for _, name := range names {
				if advertised[name] != tc.want {
					t.Errorf("%s advertised = %v, want %v", name, advertised[name], tc.want)
				}
			}
		})
	}
}

func TestSearchVaultLabelsBM25OnlyResultsAndCarriesEvidence(t *testing.T) {
	host := &vaultHost{result: vault.Result{
		Status: vault.Status{Lexical: vault.StatusReady, Semantic: vault.StatusUnavailable, Reason: "connection refused"},
		Hits: []vault.Hit{{
			ID: "s/R1-x", Ref: "personal:s/research/R1-x.md", Profile: "personal", Kind: "research", Title: "X",
			Breadcrumb: "X > Findings", Line: 12, Through: 18, Snippet: "the snippet",
			BM25: &vault.LexicalScore{Rank: 1, Score: 7.5}, FusedRank: 1, AdmittedBy: vault.AdmittedRRF,
		}},
	}}
	cs := connectVault(t, host)
	text, output := callText(t, cs, toolSearchThoughts, searchVaultInput{Query: "why", Kinds: []string{"research"}})
	for _, want := range []string{"BM25 only", "connection refused", "not relevance", "ref personal:s/research/R1-x.md", "lines 12-18", "bm25 #1 (7.5000)", "admitted by rrf", "the snippet"} {
		if !strings.Contains(text, want) {
			t.Errorf("search text lacks %q:\n%s", want, text)
		}
	}
	if hits, _ := output["hits"].([]any); len(hits) != 1 {
		t.Fatalf("structured hits = %#v", output["hits"])
	}
	if got := host.queries[0]; got.Text != "why" || len(got.Kinds) != 1 || got.Kinds[0] != "research" {
		t.Fatalf("query = %+v", got)
	}

	host.result = vault.Result{Status: vault.Status{Lexical: vault.StatusReady, Semantic: vault.StatusReady}, Hits: []vault.Hit{}}
	if text, _ := callText(t, cs, toolSearchThoughts, searchVaultInput{Query: "absent"}); !strings.Contains(text, "fresh investigation") {
		t.Fatalf("empty search text = %q", text)
	}
}

func TestOpenVaultShowsTheWholeDocumentAtItsLines(t *testing.T) {
	host := &vaultHost{excerpt: vault.Excerpt{
		Document: vault.Document{ID: "s/R1-x", Title: "X"}, Ref: "personal:s/research/R1-x.md",
		Line: 1, Through: 30, Lines: 30, Content: "# X\n",
	}}
	cs := connectVault(t, host)
	text, _ := callText(t, cs, toolOpenThoughts, openVaultInput{Ref: "s/R1-x", Line: 12, Through: 18})
	if !strings.Contains(text, "lines 12-18") {
		t.Fatalf("open text = %q", text)
	}
	if read := host.reads[0]; read.Ref != "s/R1-x" || !read.Full || read.Line != 0 {
		t.Fatalf("open read = %+v", read)
	}
	opened := host.opens[0]
	if opened.Kind != workbench.KindDocument || opened.Format != workbench.FormatMarkdown || opened.Content != "# X\n" ||
		opened.Span != (workbench.LineSpan{Line: 12, Through: 18}) || !opened.Select || opened.Source != "" {
		t.Fatalf("opened = %+v", opened)
	}

	text, _ = callText(t, cs, toolReadThoughts, readVaultInput{Ref: "s/R1-x", Line: 12})
	if !strings.HasPrefix(text, "X (s/R1-x, ref personal:s/research/R1-x.md), lines 1-30 of 30:") {
		t.Fatalf("read text = %q", text)
	}
}
