package launch

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/workbench"
)

func AssetReferences(root string, names []string) ([]workbench.AssetRef, error) {
	if len(names) == 0 {
		return nil, ErrAssetPathsRequired
	}
	refs := make([]workbench.AssetRef, 0, len(names))
	for i, name := range names {
		if name == "" {
			return nil, fmt.Errorf(assetEntryErrorFormat, i+1, name, ErrAssetPathRequired)
		}
		resolved, err := ResolveSessionFile(root, name)
		if err != nil {
			return nil, fmt.Errorf(assetEntryErrorFormat, i+1, name, err)
		}
		if _, ok := workbench.AssetMedia(resolved); !ok {
			return nil, fmt.Errorf(assetEntryErrorFormat, i+1, name, ErrUnsupportedAsset)
		}
		file, err := os.Open(resolved)
		if err != nil {
			return nil, fmt.Errorf(assetEntryErrorFormat, i+1, name, err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf(assetEntryErrorFormat, i+1, name, err)
		}
		refs = append(refs, workbench.AssetRef{Source: filepath.ToSlash(SessionRelative(root, resolved, name))})
	}
	return refs, nil
}
