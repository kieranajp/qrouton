package desktop

import (
	"bytes"
	"crypto/sha256"
	"image/png"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/vault"
)

func corpusAssetPath(source, href string) (string, bool) {
	target, _, _ := strings.Cut(href, "#")
	target, err := url.PathUnescape(target)
	parts := strings.Split(filepath.ToSlash(source), "/")
	if err != nil || len(parts) < 4 || parts[1] != "shared" || strings.ContainsAny(target, "\\\x00") || strings.HasPrefix(target, "/") || strings.Contains(target, ":") || !strings.EqualFold(path.Ext(target), ".png") {
		return "", false
	}
	resolved := path.Clean(path.Join(path.Dir(filepath.ToSlash(source)), target))
	if !strings.HasPrefix(resolved, parts[0]+"/assets/") || !filepath.IsLocal(resolved) {
		return "", false
	}
	return filepath.FromSlash(resolved), true
}

func corpusPNG(data []byte) bool {
	info, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || info.Width <= 0 || info.Height <= 0 || info.Width > 8192 || info.Height > 8192 || int64(info.Width)*int64(info.Height) > 32<<20 {
		return false
	}
	_, err = png.Decode(bytes.NewReader(data))
	return err == nil
}

func admitCorpusAssets(request *vault.CorpusRequest, selections *importSelections, keys map[string]string, total int) (map[string]importFile, error) {
	pending := map[string]importFile{}
	admitted := map[string]string{}
	payloads := map[string][]byte{}
	hashes := map[[32]byte][]byte{}
	for token, file := range selections.files {
		if file.kind == importPNG {
			admitted[file.path] = token
		}
	}
	for _, source := range request.Sources {
		file := selections.files[source.Source.Key]
		body := string(source.Source.Content)
		if override := request.Overrides[source.Source.Key]; override.Body != nil {
			body = *override.Body
		}
		for _, href := range vault.EnumerateImportAssets(body) {
			name, ok := corpusAssetPath(file.name, href)
			if !ok {
				continue
			}
			absolute := filepath.Join(file.root.Name(), name)
			token := admitted[absolute]
			data, loaded := payloads[absolute]
			asset := importFile{root: file.root, path: absolute, name: name, kind: importPNG}
			if !loaded {
				var err error
				data, err = readImportFile(asset)
				if err != nil || !corpusPNG(data) {
					continue
				}
				hash := sha256.Sum256(data)
				if shared, exists := hashes[hash]; exists {
					data = shared
				} else {
					total += len(data)
					if len(hashes) >= 256 || total > 64<<20 {
						return nil, vault.ErrImportSource
					}
					hashes[hash] = data
				}
				if token == "" {
					token, err = importToken()
					if err != nil {
						return nil, err
					}
					admitted[absolute] = token
					pending[token] = asset
				}
				payloads[absolute] = data
			}
			if len(request.Assets) >= 4096 {
				return nil, vault.ErrImportSource
			}
			request.Assets = append(request.Assets, vault.ImportAsset{Key: token, SourceKey: source.Source.Key, Href: href, Content: data})
			keys[token] = token
		}
	}
	return pending, nil
}
