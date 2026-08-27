// Package prompts owns qrouton's canonical workflow prompts and provider renderers.
package prompts

import (
	"bytes"
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

	// Skills and agents are addressed by their directory prefix; pathForID and
	// idForPath are the mapping between an ID and its file.
	skillIDPrefix = "skills/"
	agentIDPrefix = "agents/"

	skillFileName = "SKILL.md"
	promptFileExt = ".md"

	orchestratorFileName = string(Orchestrator) + promptFileExt
	assistantFileName    = string(Assistant) + promptFileExt
)

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

//go:embed orchestrator.md assistant.md subagent-choice.md agents/*.md skills/*/SKILL.md
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
	if bytes.Contains(content, []byte(subagentChoicePlaceholder)) {
		choice, err := fs.ReadFile(l.fsys, subagentChoiceFileName)
		if err != nil {
			return Prompt{}, fmt.Errorf("load prompt %q: %w", id, err)
		}
		content = bytes.ReplaceAll(content, []byte(subagentChoicePlaceholder), choice)
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
		return orchestratorFileName, nil
	case id == Assistant:
		return assistantFileName, nil
	case strings.HasPrefix(value, skillIDPrefix) && fs.ValidPath(value):
		return value + "/" + skillFileName, nil
	case strings.HasPrefix(value, agentIDPrefix) && fs.ValidPath(value):
		return value + promptFileExt, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidPromptID, id)
	}
}

func idForPath(path string) (ID, bool) {
	switch {
	case path == orchestratorFileName:
		return Orchestrator, true
	case path == assistantFileName:
		return Assistant, true
	case strings.HasPrefix(path, skillIDPrefix) && strings.HasSuffix(path, "/"+skillFileName):
		return ID(strings.TrimSuffix(path, "/"+skillFileName)), true
	case strings.HasPrefix(path, agentIDPrefix) && strings.HasSuffix(path, promptFileExt):
		return ID(strings.TrimSuffix(path, promptFileExt)), true
	default:
		return "", false
	}
}
