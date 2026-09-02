package ticket

const (
	// Provider names, used to prefix the errors a user reads.
	linearProvider = "linear"
	asanaProvider  = "asana"
	githubProvider = "github"

	linearAPIDefault = "https://api.linear.app/graphql"
	asanaAPIDefault  = "https://app.asana.com/api/1.0"
	githubAPIDefault = "https://api.github.com"

	linearHost = "linear.app"
	asanaHost  = "app.asana.com"
	githubHost = "github.com"

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

	githubIssuesSegment     = "issues"
	githubOwnerIndex        = 0
	githubRepoIndex         = 1
	githubIssuesIndex       = 2
	githubNumberIndex       = 3
	githubIssueSegments     = 4
	githubMaxReferenceBytes = 2048
	githubCanonicalFormat   = "https://github.com/%s/%s/issues/%s"
	githubIssuePathFormat   = "/repos/%s/%s/issues/%s"
	githubKeyFormat         = "%s-%s"

	pathSeparator = "/"

	linearTokenEnvVar = "LINEAR_API_KEY"
	asanaTokenEnvVar  = "ASANA_ACCESS_TOKEN"

	acceptHeader        = "Accept"
	acceptGitHubJSON    = "application/vnd.github+json"
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	contentTypeHeader   = "Content-Type"
	contentTypeJSON     = "application/json"

	requestFailedFormat    = "request failed: %s"
	decodingResponseFormat = "decoding response: %w"

	asanaTasksPath  = "/tasks/"
	asanaTaskFields = "?opt_fields=name,notes"

	// linearIssueQuery asks for exactly the two fields a session name and
	// description are built from.
	linearIssueQuery = `query Ticket($id: String!) { issue(id: $id) { title description } }`
	linearQueryKey   = "query"
	linearVarsKey    = "variables"
	linearIDVar      = "id"
)
