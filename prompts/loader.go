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

//go:embed orchestrator.md assistant.md subagent-choice.md workspace-windows.md agents/*.md all:skills
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

// read is one prompt file with the shared partials it names expanded into it.
// Every file is offered them, references included, so a skill that defers a
// passage to a partial reads the same words as the entry file.
func (l *FSLoader) read(path string) ([]byte, error) {
	content, err := fs.ReadFile(l.fsys, path)
	if err != nil {
		return nil, err
	}
	for placeholder, name := range partials {
		if !bytes.Contains(content, []byte(placeholder)) {
			continue
		}
		partial, err := fs.ReadFile(l.fsys, name)
		if err != nil {
			return nil, err
		}
		content = bytes.ReplaceAll(content, []byte(placeholder), partial)
	}
	return content, nil
}

// skillFiles is everything in a skill's folder other than its entry file, in
// lexical order.
func (l *FSLoader) skillFiles(dir string) ([]PromptFile, error) {
	entry := dir + "/" + skillFileName
	var out []PromptFile
	err := fs.WalkDir(l.fsys, dir, func(path string, node fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Hidden files are the editor's and the operating system's, never the
		// skill's. Skipping them here rather than in the embed pattern keeps a
		// working copy and the binary shipping the same skill.
		if path != dir && strings.HasPrefix(node.Name(), ".") {
			if node.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if node.IsDir() || path == entry {
			return nil
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
		id := strings.TrimSuffix(path, "/"+skillFileName)
		// Only a folder directly under skills/ is a skill. A SKILL.md deeper in
		// is one of that skill's own files, not a skill of its own.
		if strings.Contains(strings.TrimPrefix(id, skillIDPrefix), "/") {
			return "", false
		}
		return ID(id), true
	case strings.HasPrefix(path, agentIDPrefix) && strings.HasSuffix(path, promptFileExt):
		return ID(strings.TrimSuffix(path, promptFileExt)), true
	default:
		return "", false
	}
}
