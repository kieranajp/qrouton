package evalharness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/kieranajp/qrouton/prompts"
)

func CopyTree(src, dst string) error {
	return copyTree(src, dst, true)
}

func copyTree(src, dst string, preserveLinks bool) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(dst, rel)

		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if preserveLinks {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				return os.Symlink(target, destination)
			}
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("%w: %s: %w", ErrPromptLink, path, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%w: %s", ErrPromptLink, path)
			}
		}

		return copyFile(path, destination)
	})
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// MaterializeAssets stamps a hashed prompt snapshot with the production launch path.
// Evals always give the orchestrator prompt ownership of discovery files.
func MaterializeAssets(assetsDir, workspace, snapshot string) (string, error) {
	loader, hash, err := snapshotPrompts(assetsDir, snapshot)
	if err != nil {
		return "", err
	}
	if err := prompts.Stamp(context.Background(), workspace, loader, prompts.OrchestratorAsset); err != nil {
		return "", err
	}
	return hash, nil
}

func snapshotPrompts(assetsDir, snapshot string) (*prompts.FSLoader, string, error) {
	if err := copyTree(assetsDir, snapshot, false); err != nil {
		return nil, "", err
	}

	hash, err := HashTree(snapshot)
	if err != nil {
		return nil, "", err
	}
	return prompts.NewFSLoader(os.DirFS(snapshot)), hash, nil
}

func HashTree(root string) (string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)
	hash := sha256.New()
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		if _, err := fmt.Fprintln(hash, filepath.ToSlash(rel)); err != nil {
			return "", err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if _, err := hash.Write(content); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
