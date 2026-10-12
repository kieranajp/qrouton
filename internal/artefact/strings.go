package artefact

const (
	kindKey      = "kind"
	titleKey     = "title"
	authorKey    = "author"
	createdKey   = "created"
	sessionKey   = "session"
	revisionsKey = "revisions"

	titlePlaceholder = "<title>"
	untitledSlug     = "untitled"

	nameFormat     = "%s%d-%s-%s.md"
	dateFormat     = "2006-01-02"
	repoKeyFormat  = "%s/%s"
	frontmatterBar = "---\n"

	gitBin        = "git"
	gitUserName   = "user.name"
	gitConfigCmd  = "config"
	gitGlobalFlag = "--global"
	maxNameTries  = 5
	fileMode      = 0o644
	dirMode       = 0o755
)
