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
	// idForPath are the mapping between an ID and its entry file. A skill's other
	// files hang off that ID and are never IDs of their own.
	skillIDPrefix = "skills/"
	agentIDPrefix = "agents/"

	skillFileName = "SKILL.md"
	promptFileExt = ".md"

	orchestratorFileName = string(Orchestrator) + promptFileExt
	assistantFileName    = string(Assistant) + promptFileExt
)

// Prompt is one canonical prompt. Content is what a runner reads first; Files
// is what a skill folder ships beside its SKILL.md, so detail can wait until the
// entry file sends the reader after it.
type Prompt struct {
	ID      ID
	Content []byte
	Files   []PromptFile
}

// PromptFile is a supporting file inside a skill's folder, addressed relative to
// that folder.
type PromptFile struct {
	Path    string
	Content []byte
}

type PromptLoader interface {
	Load(context.Context, ID) (Prompt, error)
	List(context.Context) ([]Prompt, error)
}

type FSLoader struct{ fsys fs.FS }

func NewFSLoader(fsys fs.FS) *FSLoader { return &FSLoader{fsys: fsys} }

//go:embed orchestrator.md assistant.md subagent-choice.md agents/*.md skills
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
	content, err := l.read(path)
	if err != nil {
		return Prompt{}, fmt.Errorf("load prompt %q: %w", id, err)
	}
	prompt := Prompt{ID: id, Content: content}
	if strings.HasPrefix(string(id), skillIDPrefix) {
		prompt.Files, err = l.skillFiles(string(id))
		if err != nil {
			return Prompt{}, fmt.Errorf("load prompt %q: %w", id, err)
		}
	}
	return prompt, nil
}

// read is one prompt file with the shared subagent-choice block expanded into
// it. Every file is offered the block, references included, so a skill that
// defers the delegation guidance to one reads the same words as the entry file.
func (l *FSLoader) read(path string) ([]byte, error) {
	content, err := fs.ReadFile(l.fsys, path)
	if err != nil {
		return nil, err
	}
	if !bytes.Contains(content, []byte(subagentChoicePlaceholder)) {
		return content, nil
	}
	choice, err := fs.ReadFile(l.fsys, subagentChoiceFileName)
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(content, []byte(subagentChoicePlaceholder), choice), nil
}

// skillFiles is everything in a skill's folder other than its entry file, in
// lexical order. A skill with nothing beside SKILL.md returns none, which is how
// a single-file skill stays a single file.
func (l *FSLoader) skillFiles(dir string) ([]PromptFile, error) {
	entry := dir + "/" + skillFileName
	var out []PromptFile
	err := fs.WalkDir(l.fsys, dir, func(path string, node fs.DirEntry, walkErr error) error {
		if walkErr != nil || node.IsDir() || path == entry {
			return walkErr
		}
		content, err := l.read(path)
		if err != nil {
			return err
		}
		out = append(out, PromptFile{Path: strings.TrimPrefix(path, dir+"/"), Content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
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
