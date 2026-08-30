// Package atomicfile replaces a file whole, for the documents several qrouton
// processes write and poll.
package atomicfile

import (
	"os"
	"path/filepath"
)

// tmpSuffix ends the name of a staging file.
const tmpSuffix = ".tmp"

// Replace puts b at path whole — temp file, fsync, rename — so a poller
// re-reading it never sees a torn write and a crash leaves either document
// intact. Several processes write these files, so the temp name is unique per
// call: a shared one lets a short write land inside a long one and be renamed
// over.
func Replace(path string, b []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*"+tmpSuffix)
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
