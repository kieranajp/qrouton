package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/kieranajp/qrouton/internal/workbench"
)

type SignInState string

const (
	SignInSignedIn  SignInState = "signed-in"
	SignInSignedOut SignInState = "signed-out"
	SignInUnknown   SignInState = "unknown"
)

// SignIn carries the account's email only where the runner reports one.
type SignIn struct {
	State SignInState `json:"state"`
	Email string      `json:"email,omitempty"`
}

var unknownSignIn = SignIn{State: SignInUnknown}

// runStatus answers a status command's output and exit code; err is a command
// that never ran to an exit.
var runStatus = func(ctx context.Context, argv, env []string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	cmd.WaitDelay = statusWaitDelay
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out, exit.ExitCode(), nil
	}
	return out, 0, err
}

// PrepareProfileDir creates a profile's folder, private to the user, and leaves
// one that already exists as it is.
func PrepareProfileDir(dir string) error {
	return os.MkdirAll(dir, profileDirMode)
}

func SupportsProfiles(runnerID string) bool {
	spec, _ := specFor(runnerID)
	return spec.ProfileEnv != ""
}

// DefaultProfileDir is the folder a new profile is offered, beside the runner's
// own dot-folder in the home directory.
func DefaultProfileDir(runnerID, id string) string {
	return path.Join(profileDirHome, fmt.Sprintf(profileDirFormat, runnerID, id))
}

// CredentialsSet names the variables in environ that would sign every profile of
// the runner in, whatever its stored login.
func CredentialsSet(runnerID string, environ []string) []string {
	spec, _ := specFor(runnerID)
	set := []string{}
	for _, name := range spec.CredentialEnv {
		if slices.ContainsFunc(environ, func(entry string) bool {
			key, value, _ := strings.Cut(entry, envSep)
			return key == name && value != ""
		}) {
			set = append(set, name)
		}
	}
	return set
}

// SignInCommand is the runner's interactive login on r's profile. The default
// profile signs in with the environment qrouton inherited.
func SignInCommand(r Runner) ([]string, error) {
	spec, err := signInSpec(r)
	if err != nil {
		return nil, err
	}
	argv := append([]string{r.Path}, spec.LoginArgs...)
	if r.Profile.ID == "" {
		return argv, nil
	}
	return append([]string{envCommand, spec.ProfileEnv + envSep + r.Profile.Dir}, argv...), nil
}

// ProbeSignIn asks the runner whether r's profile is signed in. A probe that
// fails or outlives ctx answers Unknown rather than signed out.
func ProbeSignIn(ctx context.Context, r Runner) SignIn {
	spec, err := signInSpec(r)
	if err != nil {
		return unknownSignIn
	}
	env := os.Environ()
	if r.Profile.ID != "" {
		env = workbench.WithEnv(env, spec.ProfileEnv, r.Profile.Dir)
	}
	out, exit, err := runStatus(ctx, append([]string{r.Path}, spec.StatusArgs...), env)
	if err != nil || ctx.Err() != nil {
		return unknownSignIn
	}
	return spec.ReadStatus(out, exit)
}

func signInSpec(r Runner) (runnerSpec, error) {
	spec, _ := specFor(r.ID)
	if spec.ReadStatus == nil {
		return runnerSpec{}, fmt.Errorf("%w: %q", ErrProfilesUnsupported, r.ID)
	}
	if r.Path == "" {
		return runnerSpec{}, fmt.Errorf("%w: %q", ErrRunnerUnavailable, r.ID)
	}
	return spec, nil
}

// readClaudeStatus reads the JSON, since signed out also exits non-zero.
func readClaudeStatus(out []byte, _ int) SignIn {
	var status struct {
		LoggedIn bool   `json:"loggedIn"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return unknownSignIn
	}
	if !status.LoggedIn {
		return SignIn{State: SignInSignedOut}
	}
	return SignIn{State: SignInSignedIn, Email: status.Email}
}

func readCodexStatus(_ []byte, exit int) SignIn {
	switch exit {
	case 0:
		return SignIn{State: SignInSignedIn}
	case codexSignedOutExit:
		return SignIn{State: SignInSignedOut}
	}
	return unknownSignIn
}
