package desktop

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type DocumentLink struct {
	Source string `json:"source"`
	Href   string `json:"href"`
}

func (w *Windows) OpenDocumentLink(link DocumentLink) (string, error) {
	var owner *sessionState
	var source string
	var from *vault.Reference
	shown := w.shown()
	if err := w.with(link.Source, func(window *agentWindow) error {
		if window.session == nil || window.session != shown || window.opts.Kind != workbench.KindDocument {
			return vault.ErrScope
		}
		owner, source = window.session, window.opts.Source
		if window.opts.Vault != nil {
			copy := *window.opts.Vault
			from = &copy
		}
		return nil
	}); err != nil {
		return "", err
	}
	parsed, err := url.Parse(link.Href)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || strings.Contains(parsed.Path, "\\") {
		return "", vault.ErrScope
	}
	var id string
	if from != nil {
		if w.vaults == nil {
			return "", vault.ErrUnavailable
		}
		service, err := w.vaults.manager()
		if err != nil {
			return "", err
		}
		scope, err := w.vaults.scope(owner)
		if err != nil {
			return "", err
		}
		result, _, err := service.ResolveLink(scope, *from, link.Href)
		if err != nil {
			return "", err
		}
		id, err = w.showOrOpen(owner, vaultSource(result.Reference), func() (string, error) {
			return w.openStructural(owner, workbench.WindowOptions{Vault: &result.Reference})
		})
		if err != nil {
			return "", err
		}
		if _, err = w.Content(id); err != nil {
			return "", err
		}
	} else {
		target := source
		if parsed.Path != "" {
			if strings.HasPrefix(parsed.Path, "/") {
				target = strings.TrimPrefix(parsed.Path, "/")
			} else {
				target = path.Join(path.Dir(source), parsed.Path)
			}
		}
		if !filepath.IsLocal(target) || (!strings.EqualFold(path.Ext(target), ".md") && !strings.EqualFold(path.Ext(target), ".markdown")) {
			return "", vault.ErrScope
		}
		id, err = w.showOrOpen(owner, target, func() (string, error) {
			if w.newDocumentFor != nil {
				return w.newDocumentFor(owner, target)
			}
			if w.newDocument != nil {
				return w.newDocument(target)
			}
			return "", ErrNoEditorCommand
		})
		if err != nil {
			return "", err
		}
	}
	if from != nil {
		w.vaultPublishMu.Lock()
		defer w.vaultPublishMu.Unlock()
		if err := w.with(link.Source, func(window *agentWindow) error {
			if window.session != owner || window.opts.Vault == nil || *window.opts.Vault != *from {
				return vault.ErrScope
			}
			return nil
		}); err != nil {
			return "", err
		}
		if _, err := w.vaults.read(owner, *from); err != nil {
			return "", err
		}
		var target vault.Reference
		if err := w.with(id, func(window *agentWindow) error {
			if window.session != owner || window.opts.Vault == nil {
				return vault.ErrScope
			}
			target = *window.opts.Vault
			return nil
		}); err != nil {
			return "", err
		}
		fresh, err := w.vaults.read(owner, target)
		if err != nil {
			return "", err
		}
		if len(fresh.Content) > workbench.DocumentLimit {
			return "", ErrVaultDocumentLarge
		}
		if err := w.with(id, func(window *agentWindow) error {
			if window.opts.Vault == nil || *window.opts.Vault != target {
				return vault.ErrScope
			}
			window.opts.Content = fresh.Content
			return nil
		}); err != nil {
			return "", err
		}
	}
	var doc document
	if err = w.with(id, func(window *agentWindow) error {
		content, ok := window.document()
		if !ok {
			return ErrNoDocumentName
		}
		content.fragment = parsed.Fragment
		content.viewportEpoch++
		doc = documentFor(window)
		return nil
	}); err != nil {
		return "", err
	}
	w.emit(windowContentEvent+id, doc)
	return id, nil
}
