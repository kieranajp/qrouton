package vault

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type Reference struct {
	Profile string `json:"profile,omitempty"`
	ID      string `json:"id,omitempty"`
	Path    string `json:"path,omitempty"`
}
type ReadResult struct {
	Staleness string    `json:"staleness,omitempty"`
	Reference Reference `json:"reference"`
	Document  Document  `json:"document"`
	Content   string    `json:"content"`
	Path      string    `json:"-"`
	Supported bool      `json:"supported"`
}
type inventory struct {
	entries     []ReadResult
	invalid     int
	unsupported int
	conflicts   map[string]bool
}

func readFile(root *os.Root, relative string) ([]byte, error) {
	if !filepath.IsLocal(relative) || strings.Contains(relative, "\\") {
		return nil, ErrScope
	}
	file, err := root.OpenFile(relative, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxDocumentBytes {
		return nil, ErrInvalidDocument
	}
	b, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxDocumentBytes {
		return nil, ErrInvalidDocument
	}
	return b, nil
}
func scan(profile Profile) (inventory, error) {
	inv := inventory{conflicts: map[string]bool{}}
	root, err := os.OpenRoot(profile.Root)
	if err != nil {
		return inv, fmt.Errorf("%w: %s", ErrUnavailable, profile.ID)
	}
	defer root.Close()
	identities := map[string][]byte{}
	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() != "." && strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".md" && extension != ".markdown" {
			return nil
		}
		b, err := readFile(root, path)
		if err != nil {
			inv.invalid++
			return nil
		}
		d, err := Parse(b)
		if err != nil && !errors.Is(err, ErrUnsupportedSchema) {
			inv.invalid++
			return nil
		}
		supported := err == nil
		if !supported {
			inv.unsupported++
		}
		if d.ID != "" {
			if prior, ok := identities[d.ID]; ok && !bytes.Equal(prior, b) {
				inv.conflicts[d.ID] = true
			}
			identities[d.ID] = b
		}
		staleness := ""
		if unknownLegacyHistory(d) {
			staleness = "unknown"
		}
		inv.entries = append(inv.entries, ReadResult{Staleness: staleness, Reference: Reference{Profile: profile.ID, ID: d.ID, Path: filepath.ToSlash(path)}, Document: d, Content: string(b), Supported: supported})
		return nil
	})
	if err != nil {
		return inv, fmt.Errorf("%w: %s", ErrUnavailable, profile.ID)
	}
	return inv, nil
}
func resolveFile(profile Profile, result ReadResult) (ReadResult, error) {
	root, err := os.OpenRoot(profile.Root)
	if err != nil {
		return ReadResult{}, ErrUnavailable
	}
	defer root.Close()
	b, err := readFile(root, filepath.FromSlash(result.Reference.Path))
	if err != nil {
		return ReadResult{}, fmt.Errorf("%w: %v", ErrScope, err)
	}
	if string(b) != result.Content {
		return ReadResult{}, ErrConflict
	}
	realRoot, err := filepath.EvalSymlinks(profile.Root)
	if err != nil {
		return ReadResult{}, ErrUnavailable
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(profile.Root, filepath.FromSlash(result.Reference.Path)))
	if err != nil {
		return ReadResult{}, ErrUnavailable
	}
	relative, err := filepath.Rel(realRoot, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return ReadResult{}, ErrScope
	}
	result.Path = resolved
	return result, nil
}
