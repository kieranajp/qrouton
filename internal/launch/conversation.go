package launch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/config"
)

// conversationSpec is how a runner's conversation moves between profiles. A
// runner without one cannot switch profile.
type conversationSpec struct {
	// HomeDir is the state folder the default profile uses when nothing in the
	// environment moves it.
	HomeDir string
	// Locate is where the conversation at transcript belongs in the state folder stateDir.
	Locate func(transcript, stateDir string) (string, error)
	// Copy puts the conversation at transcript where Locate placed it.
	Copy func(transcript, located string) error
}

var claudeConversation = &conversationSpec{HomeDir: claudeHomeDirName, Locate: claudeTranscriptIn, Copy: copyClaudeConversation}

func SwitchesProfiles(runnerID string) bool {
	spec, _ := specFor(runnerID)
	return spec.Conversation != nil
}

// StateDir is the folder a profile keeps the runner's state in. The default
// profile has none in config, so it is the folder the runner inherits from
// environ, as a launch on it would.
func StateDir(runnerID string, profile config.RunnerProfile, environ []string) (string, error) {
	spec, _ := specFor(runnerID)
	if spec.Conversation == nil {
		return "", fmt.Errorf("%w: %q", ErrProfileSwitchUnsupported, runnerID)
	}
	if profile.ID != "" {
		return profile.Dir, nil
	}
	if dir := envValue(environ, spec.ProfileEnv); dir != "" {
		return dir, nil
	}
	home := envValue(environ, homeEnvVar)
	if home == "" {
		return "", ErrNoHome
	}
	return filepath.Join(home, spec.Conversation.HomeDir), nil
}

// TranscriptIn is where the conversation at transcript belongs in the state folder stateDir.
func TranscriptIn(runnerID, transcript, stateDir string) (string, error) {
	spec, _ := specFor(runnerID)
	if spec.Conversation == nil {
		return "", fmt.Errorf("%w: %q", ErrProfileSwitchUnsupported, runnerID)
	}
	return spec.Conversation.Locate(transcript, stateDir)
}

// CopyConversation puts the conversation at transcript into another profile's
// state folder, replacing any older copy there, and returns the copy's transcript.
func CopyConversation(runnerID, transcript, stateDir string) (string, error) {
	located, err := TranscriptIn(runnerID, transcript, stateDir)
	if err != nil {
		return "", err
	}
	spec, _ := specFor(runnerID)
	if err := spec.Conversation.Copy(transcript, located); err != nil {
		return "", err
	}
	return located, nil
}

// claudeTranscriptIn keeps the project folder name Claude chose for the transcript.
func claudeTranscriptIn(transcript, stateDir string) (string, error) {
	project := filepath.Base(filepath.Dir(transcript))
	if project == string(filepath.Separator) || project == "." || project == ".." {
		return "", fmt.Errorf("%w: %s", ErrUnplacedTranscript, transcript)
	}
	return filepath.Join(stateDir, claudeProjectsDirName, project, filepath.Base(transcript)), nil
}

// copyClaudeConversation copies the transcript and the folder beside it, which
// holds its subagents and tool results.
func copyClaudeConversation(transcript, located string) error {
	dest := filepath.Dir(located)
	if err := os.MkdirAll(dest, profileDirMode); err != nil {
		return err
	}
	info, err := os.Stat(transcript)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(transcript)
	if err != nil {
		return err
	}
	if err := atomicfile.Replace(located, content, info.Mode().Perm()); err != nil {
		return err
	}
	id := strings.TrimSuffix(filepath.Base(transcript), filepath.Ext(transcript))
	return replaceDir(filepath.Join(filepath.Dir(transcript), id), filepath.Join(dest, id))
}

// replaceDir copies source over target. os.CopyFS will not overwrite, so the
// copy lands beside target and is swapped in whole. A missing source is a
// conversation that never wrote one.
func replaceDir(source, target string) error {
	if _, err := os.Stat(source); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	parent, name := filepath.Dir(target), filepath.Base(target)
	staged, err := os.MkdirTemp(parent, name+stagedDirPattern)
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	if err := os.CopyFS(staged, os.DirFS(source)); err != nil {
		return err
	}
	retired, err := os.MkdirTemp(parent, name+retiredDirPattern)
	if err != nil {
		return err
	}
	defer os.RemoveAll(retired)
	if err := os.Rename(target, filepath.Join(retired, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(staged, target)
}

func envValue(environ []string, key string) string {
	prefix := key + envSep
	var value string
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			value = strings.TrimPrefix(entry, prefix)
		}
	}
	return value
}
