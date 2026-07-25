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
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, destination)
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

// MaterializeAssets snapshots the prompt source directory, then stamps it into
// the workspace through the same code a real launch uses, so a graded session
// sees byte-identical discovery assets. It returns the snapshot's content hash,
// which the report records to tie results to the prompts that produced them.
//
// Eval always exercises the RPI workflow, so the orchestrator prompt owns the
// discovery files regardless of what the fixture manifest says.
func MaterializeAssets(assetsDir, workspace, snapshot string) (string, error) {
	if err := CopyTree(assetsDir, snapshot); err != nil {
		return "", err
	}

	hash, err := HashTree(snapshot)
	if err != nil {
		return "", err
	}

	loader := prompts.NewFSLoader(os.DirFS(assetsDir))
	if err := prompts.Stamp(context.Background(), workspace, loader, prompts.OrchestratorAsset); err != nil {
		return "", err
	}
	return hash, nil
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
