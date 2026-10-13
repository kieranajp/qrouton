// Package artefact creates the research, spec and plan documents a session
// keeps under thoughts/shared, naming and stamping them so agents do not.
package artefact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/prompts"
)

var Kinds = []string{"research", "spec", "plan"}

// Create writes a new artefact of the given kind into the session and returns
// its path relative to the session root.
func Create(sessionRoot, kind, title string, now time.Time) (string, error) {
	if !slices.Contains(Kinds, kind) {
		return "", fmt.Errorf("%w %q: use one of %s", ErrUnknownKind, kind, strings.Join(Kinds, ", "))
	}
	folder, letter, ok := status.KindFiling(kind)
	if !ok {
		return "", fmt.Errorf("%w %q", ErrUnknownKind, kind)
	}
	template, err := prompts.Template(kind)
	if err != nil {
		return "", err
	}
	content, err := stamp(sessionRoot, template, kind, title, now)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(sessionpaths.Thoughts(sessionRoot), folder)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return "", err
	}
	dir, err = insideThoughts(sessionRoot, dir)
	if err != nil {
		return "", err
	}
	slug := capSlug(session.Slugify(title))
	if slug == "" {
		slug = untitledSlug
	}
	var name string
	err = withFolderLock(dir, func() error {
		var err error
		name, err = write(dir, letter, slug, content, now)
		return err
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(sessionpaths.ThoughtsDirName, sessionpaths.SharedDirName, folder, name), nil
}

// insideThoughts resolves the kind folder and refuses one that a link carries
// out of the session's thoughts folder.
func insideThoughts(sessionRoot, dir string) (string, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	thoughts, err := filepath.EvalSymlinks(sessionpaths.Thoughts(sessionRoot))
	if err != nil {
		return "", err
	}
	if _, inside := sessionpaths.Within(thoughts, resolved); !inside {
		return "", fmt.Errorf("%w: %s", ErrOutsideThoughts, resolved)
	}
	return resolved, nil
}

var nextNumber = status.NextNumber

// write creates the file under the folder lock, so concurrent creates cannot
// both pick the same number. The exclusive create covers a file a sync tool
// delivers meanwhile.
func write(dir, letter, slug string, content []byte, now time.Time) (string, error) {
	number := nextNumber(dir)
	for range maxNameTries {
		name := fmt.Sprintf(nameFormat, letter, number, now.Format(dateFormat), slug)
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
		if errors.Is(err, os.ErrExist) {
			number++
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.Write(content)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		return name, werr
	}
	return "", ErrNumberTaken
}

// withFolderLock serialises writers to one folder across processes. The lock
// file lives in the cache directory, so it never lands in a synced folder.
func withFolderLock(dir string, fn func() error) error {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	locks := filepath.Join(filepath.Dir(config.CachePath()), lockDirName)
	if err := os.MkdirAll(locks, dirMode); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(dir))
	return atomicfile.WithLock(filepath.Join(locks, hex.EncodeToString(sum[:8])+lockSuffix), fileMode, fn)
}

// capSlug shortens a slug to slugLimit characters, cutting at a hyphen.
func capSlug(slug string) string {
	if len(slug) <= slugLimit {
		return slug
	}
	cut := slug[:slugLimit]
	if slug[slugLimit] != '-' {
		if i := strings.LastIndex(cut, "-"); i > 0 {
			cut = cut[:i]
		}
	}
	return strings.TrimRight(cut, "-")
}

// stamp turns a template into a document: the stamped keys first, then any the
// template adds of its own, then the body with the title filled in.
func stamp(sessionRoot string, template []byte, kind, title string, now time.Time) ([]byte, error) {
	text := strings.ReplaceAll(string(template), "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, frontmatterBar)
	if !ok {
		return nil, ErrBadTemplate
	}
	head, body, ok := strings.Cut(rest, "\n"+frontmatterBar)
	if !ok {
		return nil, ErrBadTemplate
	}
	var extra yaml.Node
	if err := yaml.Unmarshal([]byte(head), &extra); err != nil {
		return nil, err
	}

	slug := filepath.Base(sessionRoot)
	revisions := yaml.Node{Kind: yaml.MappingNode}
	if m, err := session.Load(sessionRoot); err == nil {
		slug = m.Slug
		for _, repo := range m.Repos {
			if sha := revisionOf(sessionRoot, repo); sha != "" {
				revisions.Content = append(revisions.Content, scalar(fmt.Sprintf(repoKeyFormat, repo.Org, repo.Name)), scalar(sha))
			}
		}
	}

	doc := yaml.Node{Kind: yaml.MappingNode}
	add := func(key string, value *yaml.Node) { doc.Content = append(doc.Content, scalar(key), value) }
	add(kindKey, scalar(kind))
	add(titleKey, scalar(title))
	add(authorKey, scalar(author()))
	add(createdKey, scalar(now.Format(time.RFC3339)))
	add(sessionKey, scalar(slug))
	add(revisionsKey, &revisions)
	if len(extra.Content) > 0 {
		pairs := extra.Content[0].Content
		for i := 0; i+1 < len(pairs); i += 2 {
			switch pairs[i].Value {
			case kindKey, titleKey:
				continue
			}
			doc.Content = append(doc.Content, pairs[i], pairs[i+1])
		}
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(frontmatterBar)
	buf.Write(out)
	buf.WriteString(frontmatterBar)
	buf.WriteString(strings.ReplaceAll(body, titlePlaceholder, title))
	return buf.Bytes(), nil
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func revisionOf(sessionRoot string, repo session.ManifestRepo) string {
	if !repo.Role.IsEditing() {
		return repo.Revision
	}
	worktree := repo.WorktreePath
	if !filepath.IsAbs(worktree) {
		worktree = filepath.Join(sessionRoot, worktree)
	}
	sha, err := session.HeadRevision(worktree)
	if err != nil {
		return ""
	}
	return sha
}
