package main

import "errors"

const (
	appName  = "qrouton"
	appUsage = "assemble a multi-repo session and launch an agent runner in it"

	appDescription = "qrouton opens the workbench on the session you were last in, and on an\n" +
		"empty one when there are none. Sessions are assembled in the window."

	runnerFlag           = "runner"
	runnerFlagUsage      = "coding agent to launch (claude, codex, or opencode)"
	linearIssueFlag      = "linear-issue"
	linearIssueFlagUsage = "open a Linear issue in the New session flow"
	ticketFlag           = "ticket"
	ticketFlagUsage      = "open a Linear, Asana or GitHub ticket in the New session flow"
	linearPromptEnvVar   = "LINEAR_PROMPT"
	pathEnvVar           = "PATH"

	// workbenchSpecFlag is the marker the detached workbench process is started
	// with. Hidden: it is qrouton talking to itself, and the spec behind it is
	// not something a user composes. Its literal is duplicated in
	// internal/launch, which builds the argv that carries it.
	workbenchSpecFlag = "workbench-spec"

	logPrefix = appName + ":"

	// openedFormat is the whole of the successful result: the terminal is free
	// again, and the log is where to look if the window vanishes.
	openedFormat     = "opened %s — log: %s\n"
	noSessionSubject = "an empty workbench"

	macOSExecutablesDirName = "MacOS"
	macOSContentsDirName    = "Contents"
	macOSAppExtension       = ".app"
	homebrewBinDir          = "/opt/homebrew/bin"
	homebrewSbinDir         = "/opt/homebrew/sbin"
	localBinDir             = "/usr/local/bin"
	localSbinDir            = "/usr/local/sbin"
)

var bundledUserBinDirs = [][]string{
	{".local", "bin"},
	{".opencode", "bin"},
	{".bun", "bin"},
	{".cargo", "bin"},
	{".volta", "bin"},
	{".asdf", "shims"},
	{".mise", "shims"},
	{"Library", "pnpm"},
}

var (
	errWorkbenchRunning   = errors.New(`a qrouton workbench is already open — use "+ New session" in it`)
	errNoSessionArguments = errors.New("qrouton takes no arguments; assemble a session in the window")
	errLegacyWorkbench    = errors.New("the running qrouton workbench cannot open Linear issues; quit and restart it, then try again")
)
