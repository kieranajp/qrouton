package evalharness

import "errors"

// Configuration and input errors the harness refuses to start on.
var (
	ErrSamplesTooFew = errors.New("samples must be at least 1")

	ErrUnknownRunner = errors.New("runner must be " + runnerClaude + ", " + runnerCodex + ", or " + runnerAll)

	ErrRunnerUnavailable = errors.New("runner is unavailable")

	ErrScenarioNotFound = errors.New("scenario not found")

	ErrScenarioIncomplete = errors.New("id, fixture, and turns are required")

	// ErrNoEvents means a runner produced no parseable output, which is an
	// infrastructure failure rather than a graded result.
	ErrNoEvents = errors.New("runner produced no JSON events")

	// ErrNoSelfPath means the harness cannot point a runner back at itself to
	// serve the mock MCP server.
	ErrNoSelfPath = errors.New("eval executable path is empty")
)
