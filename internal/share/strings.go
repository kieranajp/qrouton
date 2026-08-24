package share

// The assembled page and the names of the parts it is built from.

const (
	scriptAsset = "assets/share.js"
	styleAsset  = "assets/share.css"

	// The publisher wraps this fragment in a document of its own, so it carries
	// no html, head or body tag. The title comes first because only the opening
	// few kilobytes are scanned for one, and the inlined fonts are long.
	titleFormat   = "<title>%s</title>\n"
	styleFormat   = "<style>\n%s</style>\n"
	payloadFormat = "<script type=\"application/json\" id=\"qrouton-document\">%s</script>\n"
	scriptFormat  = "<script>%s</script>\n"

	// headingPrefix marks the level-one heading the renderer lifts out of the
	// body and shows as the document's own title.
	headingPrefix   = "# "
	frontmatterRule = "---"

	pageSuffix    = ".html"
	slugSeparator = "-"

	dirMode  = 0o755
	fileMode = 0o644
)
