package vault

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTokenizerReference(t *testing.T) {
	data, _ := tokenizerAssets.ReadFile("assets/reference-fixtures.json")
	var fixtures struct {
		Cases []struct {
			Text   string
			Tokens []string
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixtures.Cases {
		if got := NewTokenizer().Tokens(c.Text); !reflect.DeepEqual(got, c.Tokens) {
			t.Errorf("%q: got %v want %v", c.Text, got, c.Tokens)
		}
	}
	var provenance struct {
		Files map[string]struct {
			SHA256 string `json:"sha256"`
		}
	}
	data, _ = tokenizerAssets.ReadFile("assets/provenance.json")
	if err := json.Unmarshal(data, &provenance); err != nil {
		t.Fatal(err)
	}
	for name, entry := range provenance.Files {
		b, err := tokenizerAssets.ReadFile("assets/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != entry.SHA256 {
			t.Errorf("checksum changed: %s", name)
		}
	}
}

func TestChunksPreserveSourcesAndBudget(t *testing.T) {
	body := "intro\n# Root\n## Child\n~~~go\n# code heading\n" + strings.Repeat("value := compute(input); ", 500) + "\n~~~\n```\n## also code\n```\n## Next\nend\n"
	chunks, err := ChunkDocument(body, 15)
	if err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	line := 15
	for _, c := range chunks {
		if NewTokenizer().Count(c.Input) > 256 {
			t.Fatal("budget exceeded")
		}
		if c.StartLine != line {
			t.Errorf("start %d want %d", c.StartLine, line)
		}
		end := line + strings.Count(c.Text, "\n")
		if strings.HasSuffix(c.Text, "\n") {
			end--
		}
		if c.EndLine != end {
			t.Error("wrong end line")
		}
		line += strings.Count(c.Text, "\n")
		joined.WriteString(c.Text)
		if strings.Contains(c.Text, "# code heading") || strings.Contains(c.Text, "## also code") {
			if !reflect.DeepEqual(c.Breadcrumb, []string{"Root", "Child"}) {
				t.Errorf("code became heading: %v", c.Breadcrumb)
			}
		}
	}
	if joined.String() != body {
		t.Fatal("source text lost")
	}
	if !reflect.DeepEqual(chunks[len(chunks)-1].Breadcrumb, []string{"Root", "Next"}) {
		t.Fatal("heading nesting")
	}
	_, err = ChunkDocument("# "+strings.Repeat("heading ", 300)+"\ntext", 1)
	if !errors.Is(err, ErrInputBudget) {
		t.Fatal(err)
	}
}

func fakeOllama(t *testing.T, handler http.HandlerFunc) *Ollama {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	o := NewOllama()
	transport := o.client.Transport.(*http.Transport)
	if transport.Proxy != nil {
		t.Fatal("proxy enabled")
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "127.0.0.1:11434" {
			t.Errorf("nonlocal address: %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, strings.TrimPrefix(server.URL, "http://"))
	}
	t.Cleanup(transport.CloseIdleConnections)
	return o
}
func tags(w http.ResponseWriter, digest string) {
	fmt.Fprintf(w, `{"models":[{"name":%q,"digest":%q}]}`, EmbeddingModel, digest)
}
func TestOllamaProbeAndEmbed(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "https://example.com")
	t.Setenv("HTTP_PROXY", "http://example.com")
	o := fakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			tags(w, "digest")
		case "/api/embed":
			var request struct {
				Model    string
				Input    []string
				Truncate *bool
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Model != EmbeddingModel || request.Truncate == nil || *request.Truncate {
				t.Error("wrong embedding request")
			}
			vectors := make([][]float32, len(request.Input))
			for i := range vectors {
				vectors[i] = make([]float32, 384)
				vectors[i][0] = 3
				vectors[i][1] = 4
			}
			json.NewEncoder(w).Encode(map[string]any{"embeddings": vectors})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	fp, err := o.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := o.Embed(context.Background(), fp, []string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || math.Abs(float64(vectors[0][0])-.6) > 1e-6 {
		t.Fatal(vectors)
	}
	fp.Digest = "changed"
	if _, err = o.Embed(context.Background(), fp, []string{"test"}); !errors.Is(err, ErrFingerprintChanged) {
		t.Fatal(err)
	}
}
func TestOllamaFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       error
	}{{"missing", `{"models":[]}`, 200, ErrModelMissing}, {"http", ``, 500, ErrProviderResponse}, {"redirect", ``, 302, ErrProviderResponse}, {"malformed", `{`, 200, ErrProviderResponse}} {
		t.Run(tc.name, func(t *testing.T) {
			o := fakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://example.com")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			if _, err := o.Probe(context.Background()); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	for _, v := range [][]float32{nil, make([]float32, 384), append([]float32{float32(math.Inf(1))}, make([]float32, 383)...), append([]float32{float32(math.NaN())}, make([]float32, 383)...)} {
		if !errors.Is(NormalizeVector(v), ErrInvalidVector) {
			t.Fatal("invalid vector accepted")
		}
	}
	o := fakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			tags(w, "d")
		} else {
			io.WriteString(w, `{"embeddings":[[1,2]]}`)
		}
	})
	if _, err := o.Probe(context.Background()); !errors.Is(err, ErrInvalidVector) {
		t.Fatal(err)
	}
}
func TestOllamaPullProgressAndCancellation(t *testing.T) {
	o := fakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pull" {
			t.Error(r.URL.Path)
		}
		io.WriteString(w, "{\"status\":\"downloading\",\"total\":10,\"completed\":5}\n{\"status\":\"success\"}\n")
	})
	var events []PullProgress
	if err := o.Pull(context.Background(), func(p PullProgress) { events = append(events, p) }); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Completed != 5 {
		t.Fatal(events)
	}
	blocked := fakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := blocked.Pull(ctx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestEmbeddingBudgetCheckedBeforeNetwork(t *testing.T) {
	provider := fakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("over-budget input reached HTTP")
		w.WriteHeader(500)
	})
	if _, err := provider.Embed(context.Background(), Fingerprint{}, []string{strings.Repeat("hello ", 255)}); !errors.Is(err, ErrInputBudget) {
		t.Fatal(err)
	}
	for _, control := range []string{"\v", "\f", "\u0085"} {
		input := strings.Repeat("hello"+control+"hello ", 100)
		if got := NewTokenizer().Count(input); got != 302 {
			t.Fatalf("control %q: got %d tokens", control, got)
		}
		if _, err := provider.Embed(context.Background(), Fingerprint{}, []string{input}); !errors.Is(err, ErrInputBudget) {
			t.Fatal(err)
		}
	}
}

func TestOllamaRejectsRemoteModelMetadataBeforeEmbedding(t *testing.T) {
	for _, field := range []string{"remote_model", "remote_host"} {
		t.Run(field, func(t *testing.T) {
			provider := fakeOllama(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/tags" {
					t.Error("remote model reached embedding endpoint")
				}
				json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": EmbeddingModel, "digest": "digest", field: "remote"}}})
			})
			if _, err := provider.Probe(context.Background()); !errors.Is(err, ErrProviderResponse) {
				t.Fatal(err)
			}
		})
	}
}
