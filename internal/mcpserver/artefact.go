package mcpserver

import (
	"fmt"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/artefact"
)

func createArtefact(root string, input createArtefactInput) (string, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return "", ErrArtefactTitle
	}
	path, err := artefact.Create(root, strings.ToLower(strings.TrimSpace(input.Kind)), title, time.Now())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(artefactCreatedFormat, path), nil
}
