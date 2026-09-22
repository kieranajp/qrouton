package vault

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type ImportAsset struct {
	Key       string `json:"key"`
	SourceKey string `json:"sourceKey"`
	Href      string `json:"href"`
	Content   []byte `json:"-"`
}
type ImportAssetInfo struct {
	Key    string `json:"key"`
	Href   string `json:"href"`
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Size   int    `json:"size"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func EnumerateImportAssets(body string) []string {
	if strings.HasPrefix(body, "---\n") {
		if _, text, err := legacyFields([]byte(body)); err == nil {
			body = text
		}
	}
	found := map[string]bool{}
	index := &importLinkIndex{aliases: map[string]map[string]ImportEntry{}, rewrite: func(href string) (string, bool) {
		target, _, _ := strings.Cut(href, "#")
		decoded, err := url.PathUnescape(target)
		if err == nil && !linkScheme.MatchString(decoded) && !strings.HasPrefix(decoded, "//") && strings.EqualFold(path.Ext(decoded), ".png") {
			found[href] = true
		}
		return href, true
	}}
	normalizeImportLinksIndexed(body, ImportSource{}, ImportEntry{}, nil, index)
	result := []string{}
	for href := range found {
		result = append(result, href)
	}
	sort.Strings(result)
	return result
}
func prepareImportAssets(ctx context.Context, record *importRecord, assets []ImportAsset) error {
	if len(assets) > 4096 {
		return ErrImportSource
	}
	record.Assets = map[string][]byte{}
	entries := map[string]*ImportEntry{}
	sources := map[string]ImportSource{}
	for i := range record.Preview.Entries {
		entries[record.Preview.Entries[i].Key] = &record.Preview.Entries[i]
	}
	for _, source := range record.Sources {
		sources[source.Key] = source
	}
	total := 0
	manifests := map[string]bool{}
	for _, source := range record.Sources {
		total += len(source.Content)
		if source.Session != nil && !manifests[source.Session.Key] {
			manifests[source.Session.Key] = true
			total += len(source.Session.Content)
		}
	}
	for _, asset := range assets {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := entries[asset.SourceKey]
		source := sources[asset.SourceKey]
		if entry == nil || asset.Key == "" || len(asset.Content) > 8<<20 || len(asset.Content) == 0 {
			return ErrImportSource
		}
		hash := contentHash(string(asset.Content))
		if prior, ok := record.Hashes[asset.Key]; ok && prior != hash {
			return ErrImportSource
		}
		for _, src := range record.Sources {
			if src.Key == asset.Key || src.Session != nil && src.Session.Key == asset.Key {
				return ErrImportSource
			}
		}
		admitted := false
		for _, href := range append(EnumerateImportAssets(entry.Document.Body), EnumerateImportAssets(string(source.Content))...) {
			if href == asset.Href {
				admitted = true
			}
		}
		target, _, _ := strings.Cut(asset.Href, "#")
		target, err := url.PathUnescape(target)
		parts := strings.Split(source.RelativePath, "/")
		resolved := path.Clean(path.Join(path.Dir(source.RelativePath), target))
		if !admitted || err != nil || len(parts) < 4 || strings.HasPrefix(target, "/") || !strings.HasPrefix(resolved, parts[0]+"/assets/") {
			return ErrImportSource
		}
		config, err := png.DecodeConfig(bytes.NewReader(asset.Content))
		if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 32<<20 {
			return ErrImportSource
		}
		if _, err := png.Decode(bytes.NewReader(asset.Content)); err != nil {
			return ErrImportSource
		}
		if _, ok := record.Assets[hash]; !ok {
			total += len(asset.Content)
			record.Assets[hash] = asset.Content
		}
		if len(record.Assets) > 256 || total > 64<<20 {
			return ErrImportSource
		}
		record.Hashes[asset.Key] = hash
		info := ImportAssetInfo{Key: asset.Key, Href: asset.Href, Path: entry.Document.Session + "/assets/" + hash + ".png", Hash: hash, Size: len(asset.Content), Width: config.Width, Height: config.Height}
		entry.Assets = append(entry.Assets, info)
	}
	return nil
}
func assetLink(entry ImportEntry, href string) (string, bool) {
	for _, asset := range entry.Assets {
		if asset.Href == href {
			relative, err := filepath.Rel(filepath.Dir(entry.Path), asset.Path)
			if err != nil {
				return "", false
			}
			_, fragment, _ := strings.Cut(href, "#")
			result := escapeMarkdownPath(filepath.ToSlash(relative))
			if fragment != "" {
				result += "#" + fragment
			}
			return result, true
		}
	}
	return "", false
}
func writeImportAssets(ctx context.Context, root, store *os.Root, job queueEntry, session string) error {
	if len(job.Assets) == 0 {
		return nil
	}
	record, err := loadImportRecord(store, job.Preview)
	if err != nil {
		return ErrImportRepair
	}
	namespace, err := openChildDirectory(root, session, true)
	if err != nil {
		return ErrImportWrite
	}
	defer namespace.Close()
	directory, err := openChildDirectory(namespace, "assets", true)
	if err != nil {
		return ErrImportWrite
	}
	defer directory.Close()
	for _, asset := range job.Assets {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, ok := record.Assets[asset.Hash]
		if !ok || contentHash(string(data)) != asset.Hash || asset.Path != session+"/assets/"+asset.Hash+".png" {
			return ErrImportRepair
		}
		reviewed := false
		for _, entry := range record.Preview.Entries {
			if entry.Key == job.Source {
				for _, known := range entry.Assets {
					if known == asset {
						reviewed = true
					}
				}
			}
		}
		if !reviewed {
			return ErrImportRepair
		}
		if err := writeOnce(directory, asset.Hash+".png", data); err != nil {
			if errors.Is(err, ErrConflict) {
				return ErrImportRepair
			}
			return ErrImportWrite
		}
	}
	return nil
}
