// Package prompts owns qrouton's canonical workflow prompts and provider renderers.
package prompts

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

type ID string

const (
	Orchestrator ID = "orchestrator"
	Assistant    ID = "assistant"
)

func Skill(name string) ID { return ID("skills/" + name) }
func Agent(name string) ID { return ID("agents/" + name) }

type Prompt struct {
	ID      ID
	Content []byte
}

type PromptLoader interface {
	Load(context.Context, ID) (Prompt, error)
	List(context.Context) ([]Prompt, error)
}

type FSLoader struct{ fsys fs.FS }

func NewFSLoader(fsys fs.FS) *FSLoader { return &FSLoader{fsys: fsys} }

//go:embed orchestrator.md assistant.md agents/*.md skills/*/SKILL.md
var embedded embed.FS

func NewEmbeddedLoader() PromptLoader { return NewFSLoader(embedded) }

func (l *FSLoader) Load(ctx context.Context, id ID) (Prompt, error) {
	if err := ctx.Err(); err != nil {
		return Prompt{}, err
	}
	path, err := pathForID(id)
	if err != nil {
		return Prompt{}, err
	}
	content, err := fs.ReadFile(l.fsys, path)
	if err != nil {
		return Prompt{}, fmt.Errorf("load prompt %q: %w", id, err)
	}
	return Prompt{ID: id, Content: content}, nil
}

func (l *FSLoader) List(ctx context.Context) ([]Prompt, error) {
	var ids []ID
	err := fs.WalkDir(l.fsys, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		id, ok := idForPath(path)
		if ok {
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]Prompt, 0, len(ids))
	for _, id := range ids {
		prompt, err := l.Load(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, prompt)
	}
	return out, nil
}

func pathForID(id ID) (string, error) {
	value := string(id)
	switch {
	case id == Orchestrator:
		return "orchestrator.md", nil
	case id == Assistant:
		return "assistant.md", nil
	case strings.HasPrefix(value, "skills/") && fs.ValidPath(value):
		return value + "/SKILL.md", nil
	case strings.HasPrefix(value, "agents/") && fs.ValidPath(value):
		return value + ".md", nil
	default:
		return "", fmt.Errorf("invalid prompt id %q", id)
	}
}

func idForPath(path string) (ID, bool) {
	switch {
	case path == "orchestrator.md":
		return Orchestrator, true
	case path == "assistant.md":
		return Assistant, true
	case strings.HasPrefix(path, "skills/") && strings.HasSuffix(path, "/SKILL.md"):
		return ID(strings.TrimSuffix(path, "/SKILL.md")), true
	case strings.HasPrefix(path, "agents/") && strings.HasSuffix(path, ".md"):
		return ID(strings.TrimSuffix(path, ".md")), true
	default:
		return "", false
	}
}
