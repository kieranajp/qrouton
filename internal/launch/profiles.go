package launch

import "os"

// PrepareProfileDir creates a profile's folder, private to the user, and leaves
// one that already exists as it is.
func PrepareProfileDir(dir string) error {
	return os.MkdirAll(dir, profileDirMode)
}
