package ticket

// The two ticket providers qrouton reads: their API endpoints, the URL shapes it
// accepts, and the credentials each needs.

const (
	linearAPIDefault = "https://api.linear.app/graphql"
	asanaAPIDefault  = "https://app.asana.com/api/1.0"

	linearHost = "linear.app"
	asanaHost  = "app.asana.com"

	httpsScheme = "https"

	// A Linear issue URL is /<workspace>/issue/<id>/<slug>.
	linearIssueSegment = "issue"
	linearIssueIndex   = 2
	linearMinSegments  = 3

	// An Asana task URL is /0/<project>/<task>.
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
