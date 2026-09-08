package launch

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/workbench"
)

func ImageReferences(root string, names []string) ([]workbench.ImageRef, error) {
	if len(names) == 0 {
		return nil, ErrImagePathsRequired
	}
	refs := make([]workbench.ImageRef, 0, len(names))
	for i, name := range names {
		if name == "" {
			return nil, fmt.Errorf(imageEntryErrorFormat, i+1, name, ErrImagePathRequired)
		}
		resolved, err := ResolveSessionFile(root, name)
		if err != nil {
			return nil, fmt.Errorf(imageEntryErrorFormat, i+1, name, err)
		}
		if _, ok := workbench.RasterMediaType(resolved); !ok {
			return nil, fmt.Errorf(imageEntryErrorFormat, i+1, name, ErrUnsupportedImage)
		}
		file, err := os.Open(resolved)
		if err != nil {
			return nil, fmt.Errorf(imageEntryErrorFormat, i+1, name, err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf(imageEntryErrorFormat, i+1, name, err)
		}
		refs = append(refs, workbench.ImageRef{Source: filepath.ToSlash(SessionRelative(root, resolved, name))})
	}
	return refs, nil
}
