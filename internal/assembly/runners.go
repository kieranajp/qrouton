package assembly

// Runner is one coding agent as the agent step offers it. Installed means
// launch resolved a path for it; main.go maps launch's own rows onto these so
// nothing here imports launch.
type Runner struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Installed bool   `json:"installed"`
}

// Installed keeps only the runners that can actually be started, which is what
// "only agents found on your PATH are listed" means.
func Installed(runners []Runner) []Runner {
	out := make([]Runner, 0, len(runners))
	for _, r := range runners {
		if r.Installed {
			out = append(out, r)
		}
	}
	return out
}
