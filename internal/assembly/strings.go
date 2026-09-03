package assembly

const (
	msgNameRequired  = "A session name is needed."
	msgNoEditingRepo = "At least one editing repo is needed."
	msgSessionExists = "A session is already assembled under that folder name."

	branchFormat = "%s/%s"

	repositoryNoticeFormat          = "qrouton: session repositories changed — %s. Re-read qrouton.json before continuing with the updated workspace."
	repositoryNoticeAddedEditing    = "added %s for editing at %s"
	repositoryNoticeAddedReference  = "added %s as a read-only reference at %s"
	repositoryNoticePromotedEditing = "promoted %s to editing at %s"
	repositoryNoticeSeparator       = "; "
)

const (
	branchDescriptionMaxWords       = 4
	branchDescriptionMaxLength      = 32
	branchDescriptionMinClauseWords = 2
)

var (
	branchTitleSeparators = []string{":", " — ", " - "}
	branchArticles        = map[string]bool{"a": true, "an": true, "the": true}
)

const (
	OutcomeDraft    = "draft"
	OutcomeExisting = "existing-session"
	OutcomeQueued   = "queued"
)
