package vault

import (
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

type stamp struct {
	size  int64
	mtime int64
}

// walk stamps every Markdown file under dir. os.Root follows a symlink only
// while it stays inside dir, so an escaping link is simply not a document.
func walk(dir string) (map[string]stamp, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	stamps := map[string]stamp{}
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			if name == "." {
				return err
			}
			return nil
		}
		if name != "." && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !markdownFile(name) || syncArtefact(entry.Name()) {
			return nil
		}
		if info, err := documentInfo(root, name); err == nil {
			stamps[name] = stamp{size: info.Size(), mtime: info.ModTime().UnixNano()}
		}
		return nil
	})
	return stamps, err
}

func markdownFile(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// syncArtefact reports a copy a sync tool made beside a real file.
func syncArtefact(base string) bool {
	return strings.Contains(base, ".sync-conflict-") || strings.HasPrefix(base, "~syncthing~") ||
		strings.Contains(base, " (conflicted copy") || strings.Contains(base, " (Conflict")
}

func documentInfo(root *os.Root, name string) (fs.FileInfo, error) {
	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxDocumentBytes {
		return nil, ErrNotDocument
	}
	return info, nil
}

func readDocument(dir, name string) ([]byte, error) {
	if !fs.ValidPath(name) || !markdownFile(name) {
		return nil, ErrNotDocument
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer root.Close()
	if _, err := documentInfo(root, name); err != nil {
		return nil, err
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	b, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxDocumentBytes {
		return nil, ErrNotDocument
	}
	return b, nil
}
