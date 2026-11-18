package session

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// Conversation is the root conversation a runner last reported: the ID a resume
// names, and the transcript a profile switch copies.
type Conversation struct {
	ID         string `json:"id"`
	Transcript string `json:"transcript"`
}

// conversationID is the shape of a runner's conversation ID. The ID reaches
// argv and a file name, so it may not start a flag or leave a folder.
var conversationID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func (c Conversation) valid() bool {
	return conversationID.MatchString(c.ID) && filepath.IsAbs(c.Transcript) &&
		filepath.Clean(c.Transcript) == c.Transcript && filepath.Base(c.Transcript) == c.ID+transcriptExt
}

func RecordConversation(root string, c Conversation) error {
	if !c.valid() {
		return ErrInvalidConversation
	}
	if err := os.MkdirAll(sessionpaths.Dir(root), dirMode); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return atomicfile.Replace(sessionpaths.Conversation(root), b, fileMode)
}

// ReadConversation reports no conversation for a missing, unreadable or
// malformed record alike.
func ReadConversation(root string) (Conversation, bool) {
	b, err := os.ReadFile(sessionpaths.Conversation(root))
	if err != nil {
		return Conversation{}, false
	}
	var c Conversation
	if json.Unmarshal(b, &c) != nil || !c.valid() {
		return Conversation{}, false
	}
	return c, true
}

func ClearConversation(root string) error {
	if err := os.Remove(sessionpaths.Conversation(root)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
