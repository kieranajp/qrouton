package assembly

const (
	msgNameRequired  = "A name is needed — it becomes the folder and the branch."
	msgNoEditingRepo = "At least one editing repo is needed."
	msgSessionExists = "A session is already assembled under that name."

	branchFormat = "%s/%s"

	repositoryNoticeFormat          = "qrouton: session repositories changed — %s. Re-read qrouton.json before continuing with the updated workspace."
	repositoryNoticeAddedEditing    = "added %s for editing at %s"
	repositoryNoticeAddedReference  = "added %s as a read-only reference at %s"
	repositoryNoticePromotedEditing = "promoted %s to editing at %s"
	repositoryNoticeSeparator       = "; "
)
