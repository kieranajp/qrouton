package desktop

import (
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type galleryAssetRef struct{ root, source string }

func admittedWindowOptions(owner *sessionState, opts workbench.WindowOptions) (workbench.WindowOptions, error) {
	if opts.Format != workbench.FormatAssets {
		if len(opts.Assets) != 0 {
			return opts, ErrInvalidAssetGallery
		}
		return opts, nil
	}
	if owner == nil || opts.Kind != workbench.KindDocument || opts.Source != "" || opts.Deck || len(opts.Command) != 0 {
		return opts, ErrInvalidAssetGallery
	}
	names := make([]string, len(opts.Assets))
	for i := range opts.Assets {
		names[i] = opts.Assets[i].Source
	}
	refs, err := launch.AssetReferences(owner.root(), names)
	if err != nil {
		return opts, fmt.Errorf("%w: %w", ErrInvalidAssetGallery, err)
	}
	opts.Assets = refs
	return opts, nil
}

func assetManifest(refs []workbench.AssetRef, current int) string {
	var out strings.Builder
	for i, ref := range refs {
		fmt.Fprintf(&out, "%d. %s\n", i+1, ref.Source)
	}
	fmt.Fprintf(&out, currentAssetFormat, current, len(refs))
	return out.String()
}

func (r *registry) lookupGalleryAsset(token string, index int) (galleryAssetRef, bool) {
	if token == "" || index < 1 {
		return galleryAssetRef{}, false
	}
	var ref galleryAssetRef
	var found bool
	r.each(func(_ string, window *agentWindow) {
		if r.assetsClosed || r.assetOwnersClosed[window.session] || found || window.asset != token || window.opts.Format != workbench.FormatAssets {
			return
		}
		doc, ok := window.document()
		if !ok || index > len(doc.assets) || window.session == nil || window.session.root() == "" {
			return
		}
		ref, found = galleryAssetRef{root: window.session.root(), source: doc.assets[index-1].Source}, true
	})
	return ref, found
}

func (r *registry) focusAsset(owner *sessionState, id string, index int) (workbench.AssetSelection, error) {
	r.assetFocusMu.Lock()
	defer r.assetFocusMu.Unlock()
	var selected workbench.AssetSelection
	var doc document
	err := r.with(id, func(window *agentWindow) error {
		if r.assetsClosed || r.assetOwnersClosed[owner] || owner == nil || window.session != owner {
			return noSuchWindow(id)
		}
		rendered, ok := window.document()
		if !ok || window.opts.Format != workbench.FormatAssets {
			return ErrNotAssetGallery
		}
		if index < 1 || index > len(rendered.assets) {
			return fmt.Errorf(assetIndexErrorFormat, ErrAssetIndex, index, len(rendered.assets))
		}
		rendered.currentAsset = index
		rendered.assetRevision++
		r.selected[owner] = id
		selected = workbench.AssetSelection{CurrentIndex: index, Count: len(rendered.assets), Revision: rendered.assetRevision}
		doc = documentFor(window)
		return nil
	})
	if err != nil {
		return workbench.AssetSelection{}, err
	}
	r.announce(owner)
	r.emit(windowContentEvent+id, doc)
	return selected, nil
}
