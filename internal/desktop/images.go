package desktop

import (
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type imageAssetRef struct{ root, source string }

func admittedWindowOptions(owner *sessionState, opts workbench.WindowOptions) (workbench.WindowOptions, error) {
	if opts.Format != workbench.FormatImages {
		if len(opts.Images) != 0 {
			return opts, ErrInvalidImageGallery
		}
		return opts, nil
	}
	if owner == nil || opts.Kind != workbench.KindDocument || opts.Source != "" || opts.Deck || len(opts.Command) != 0 {
		return opts, ErrInvalidImageGallery
	}
	names := make([]string, len(opts.Images))
	for i := range opts.Images {
		names[i] = opts.Images[i].Source
	}
	refs, err := launch.ImageReferences(owner.root(), names)
	if err != nil {
		return opts, fmt.Errorf("%w: %w", ErrInvalidImageGallery, err)
	}
	opts.Images = refs
	return opts, nil
}

func imageManifest(refs []workbench.ImageRef, current int) string {
	var out strings.Builder
	for i, ref := range refs {
		fmt.Fprintf(&out, "%d. %s\n", i+1, ref.Source)
	}
	fmt.Fprintf(&out, currentImageFormat, current, len(refs))
	return out.String()
}

func (r *registry) imageAsset(token string, index int) (imageAssetRef, bool) {
	if token == "" || index < 1 {
		return imageAssetRef{}, false
	}
	var ref imageAssetRef
	var found bool
	r.each(func(_ string, window *agentWindow) {
		if found || window.asset != token || window.opts.Format != workbench.FormatImages {
			return
		}
		doc, ok := window.document()
		if !ok || index > len(doc.images) || window.session == nil || window.session.root() == "" {
			return
		}
		ref, found = imageAssetRef{root: window.session.root(), source: doc.images[index-1].Source}, true
	})
	return ref, found
}
