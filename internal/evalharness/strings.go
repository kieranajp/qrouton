package evalharness

// Literal values the harness matches on or shells out to. Runner names, check
// kinds, and manifest roles are contract with the scenario files and the
// session manifest, so they live here rather than inline at each use.

const (
	gitBin     = "git"
	srcDirName = "src"

	roleReference = "reference"
)
