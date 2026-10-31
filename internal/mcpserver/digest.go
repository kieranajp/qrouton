package mcpserver

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kieranajp/qrouton/internal/session"
)

func setDigest(root string, input setDigestInput) (string, error) {
	text := strings.TrimSpace(input.Digest)
	count := utf8.RuneCountInString(text)
	switch {
	case count == 0:
		return "", ErrDigestRequired
	case count > digestLimit:
		return "", digestTooLong(count)
	}
	err := session.UpdateManifest(root, func(m session.Manifest) (session.Manifest, error) {
		m.Digest = session.Digest{Text: text, At: time.Now().UTC()}
		return m, nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(digestSetFormat, count, digestLimit), nil
}
