package mcpserver

import (
	"fmt"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/artefact"
)

func createArtefact(root string, input createArtefactInput, templateFor func(string) ([]byte, error)) (string, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return "", ErrArtefactTitle
	}
	path, err := artefact.CreateWithTemplates(root, strings.ToLower(strings.TrimSpace(input.Kind)), title, time.Now(), templateFor)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(artefactCreatedFormat, path), nil
}
