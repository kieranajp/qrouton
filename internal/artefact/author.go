package artefact

import (
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
)

var (
	configAuthor = func() string {
		cfg, err := config.Load()
		if err != nil {
			return ""
		}
		return cfg.Author
	}
	gitAuthor = func() string {
		out, err := exec.Command(gitBin, gitConfigCmd, gitGlobalFlag, gitUserName).Output()
		if err != nil {
			return ""
		}
		return string(out)
	}
	osAuthor = func() string {
		if u, err := user.Current(); err == nil && u.Username != "" {
			return u.Username
		}
		return os.Getenv("USER")
	}
)

// author is the configured name, then git's, then the login name. It may be
// empty, but never fails.
func author() string {
	for _, source := range []func() string{configAuthor, gitAuthor, osAuthor} {
		if name := strings.TrimSpace(source()); name != "" {
			return name
		}
	}
	return ""
}
