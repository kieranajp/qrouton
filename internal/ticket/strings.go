package ticket

const (
	linearAPIDefault = "https://api.linear.app/graphql"
	asanaAPIDefault  = "https://app.asana.com/api/1.0"

	linearHost = "linear.app"
	asanaHost  = "app.asana.com"

	httpsScheme = "https"

	linearIssueSegment       = "issue"
	linearShortIDIndex       = 1
	linearScopedIDIndex      = 2
	linearShortMinSegments   = 2
	linearScopedMinSegments  = 3
	linearCanonicalPrefix    = "https://linear.app/issue/"
	linearMaxReferenceBytes  = 2048
	linearMaxIdentifierBytes = 128

	asanaRootSegment = "0"
	asanaMinSegments = 3

	pathSeparator = "/"

	linearTokenEnvVar = "LINEAR_API_KEY"
	asanaTokenEnvVar  = "ASANA_ACCESS_TOKEN"

	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	contentTypeHeader   = "Content-Type"
	contentTypeJSON     = "application/json"

	asanaTasksPath  = "/tasks/"
	asanaTaskFields = "?opt_fields=name,notes"

	// linearIssueQuery asks for exactly the two fields a session name and
	// description are built from.
	linearIssueQuery = `query Ticket($id: String!) { issue(id: $id) { title description } }`
	linearQueryKey   = "query"
	linearVarsKey    = "variables"
	linearIDVar      = "id"
)
