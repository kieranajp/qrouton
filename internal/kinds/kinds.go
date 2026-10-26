// Package kinds is the artefact taxonomy: the folder each kind is filed under
// in thoughts/shared, its name, and the letter its filenames are numbered from.
package kinds

import "strings"

type Kind struct {
	Dir      string
	Name     string
	Letter   string
	Template bool
}

var (
	Research  = Kind{Dir: "research", Name: "research", Letter: "r", Template: true}
	Spec      = Kind{Dir: "specs", Name: "spec", Letter: "s", Template: true}
	Plan      = Kind{Dir: "plans", Name: "plan", Letter: "p", Template: true}
	Explainer = Kind{Dir: "explainers", Name: "explainer", Letter: "e"}
	Audit     = Kind{Dir: "audits", Name: "audit", Letter: "a", Template: true}
)

func All() []Kind {
	return []Kind{Research, Spec, Plan, Explainer, Audit}
}

// Label is the kind as the chrome and the page spell it.
func (k Kind) Label() string {
	return strings.ToUpper(k.Name)
}

// Templated is the kinds a session scaffolds and the agent can create.
func Templated() []Kind {
	var out []Kind
	for _, k := range All() {
		if k.Template {
			out = append(out, k)
		}
	}
	return out
}
