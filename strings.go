package main

import "errors"

// CLI identity and help text, and the messages the headless ad-hoc path prints
// while it works.

const (
	appName      = "qrouton"
	appUsage     = "assemble a multi-repo session and launch an agent runner in it"
	appArgsUsage = "[dir | owner/repo ...]"

	appDescription = "With no arguments, qrouton opens the session list. Given a directory,\n" +
		"it drops straight into a fresh zero-repo scratch session in Assistant\n" +
		"mode, named after it — no picker, no network. Given one or more\n" +
		"owner/repo arguments, it launches (or resumes) an ad-hoc Assistant-mode\n" +
		"session with those repos active — e.g. `qrouton kieranajp/qrouton`."

	refreshFlag      = "refresh"
	refreshFlagUsage = "refresh the cached org repo list"
	runnerFlag       = "runner"
	runnerFlagUsage  = "coding agent to launch (claude, codex, or opencode)"

	// workbenchSpecFlag is the marker the detached workbench process is started
	// with. Hidden: it is qrouton talking to itself, and the spec behind it is
	// not something a user composes. Its literal is duplicated in
	// internal/launch, which builds the argv that carries it.
	workbenchSpecFlag = "workbench-spec"

	// The ad-hoc path has no TUI, so it narrates to stderr.
	logPrefix      = appName + ":"
	resumingFormat = logPrefix + " resuming %s\n"
	progressFormat = logPrefix + " %s %s\n"

	// openedFormat is the whole of the successful result: the terminal is free
	// again, and the log is where to look if the windows vanish.
	openedFormat       = "opened %s — log: %s\n"
	sessionListSubject = "the session list"

	// adhocBranchPrefix names the branch of an ad-hoc session's active repos.
	// These sessions start without a ticket, so "chore" reads more honestly
	// than "feat".
	adhocBranchPrefix = "chore"

	// adhocNameSeparator joins repository names into a session name.
	adhocNameSeparator = "-"

	repoSpecSeparator = "/"
	repoSpecParts     = 2
	gitDirSuffix      = ".git"
)

var (
	errNoRepositories = errors.New("no repositories given")
	errRepoSpecShape  = errors.New("expected owner/repo")
)
