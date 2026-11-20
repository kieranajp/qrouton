package github

import "time"

// GitHub GraphQL enum values a pull request reports.
const (
	PullStateOpen   = "OPEN"
	PullStateClosed = "CLOSED"
	PullStateMerged = "MERGED"

	ReviewApproved         = "APPROVED"
	ReviewChangesRequested = "CHANGES_REQUESTED"

	CheckFailure = "FAILURE"
	CheckError   = "ERROR"
)

const (
	graphQLPath          = "/graphql"
	contentTypeHeader    = "Content-Type"
	jsonContentType      = "application/json"
	graphQLResponseLimit = 8 << 20

	rateLimitRemainingHeader = "x-ratelimit-remaining"
	rateLimitResetHeader     = "x-ratelimit-reset"
	retryAfterHeader         = "retry-after"
	rateLimitedType          = "RATE_LIMITED"
	secondaryLimitPhrase     = "secondary rate limit"
	rateLimitFallback        = time.Minute
	rateLimitedFormat        = "%v until %s"
	// requestIDPattern is the quoted id in GitHub's generic error, such as `C0A1:2B3C:4D5E6F:7A8B9C:65A1B2C3`.
	requestIDPattern = "`[0-9A-Fa-f]+(?::[0-9A-Fa-f]+){3,}`"

	notFoundType = "NOT_FOUND"

	// Every target string travels as a variable; the query text holds only indices.
	pullsQueryFormat     = "query(%s) { viewer { login } %s }"
	pullsVariablesFormat = "$o%[1]d: String!, $n%[1]d: String!, $b%[1]d: String!"
	pullsAliasFormat     = "%[1]s: repository(owner: $o%[2]d, name: $n%[2]d) { pullRequests(headRefName: $b%[2]d, first: %[3]d, orderBy: {field: UPDATED_AT, direction: DESC}) { nodes { " + pullNodeFields + " } } } "
	pullNodeFields       = "number title url state isDraft updatedAt author { login } latestOpinionatedReviews(first: 20, writersOnly: true) { nodes { state } } headRepositoryOwner { login } statusCheckRollup { state }"
	aliasFormat          = "r%d"
	viewerField          = "viewer"
	ownerVariableFormat  = "o%d"
	nameVariableFormat   = "n%d"
	branchVariableFormat = "b%d"
	pullsPageSize        = 50
	variableSeparator    = ", "
)

const (
	IssueRepository     = "kieranajp/qrouton"
	IssueListURL        = WebBase + "/" + IssueRepository + "/issues"
	issueAPIURL         = APIBaseDefault + "/repos/" + IssueRepository + "/issues"
	issueResponseLimit  = 1 << 20
	issueRejectedFormat = "%w (HTTP %d): check GitHub access and the report, then propose it again"

	// APIBaseDefault is overridable in tests via githubAPIBase.
	APIBaseDefault = "https://api.github.com"
	WebBase        = "https://github.com"

	usersPath = "/users/"
	orgsPath  = "/orgs/"
	reposPath = "/repos"
	userPath  = "/user"

	// userReposQuery lists the authenticated user's own repositories, including
	// private ones — the /users/<login>/repos endpoint exposes only public.
	userReposQuery         = "/user/repos?affiliation=owner&visibility=all"
	collaboratorReposQuery = "/user/repos?affiliation=collaborator&visibility=all"
	orgReposQuery          = "?type=all"
	otherUserQuery         = "?type=owner"

	// branchesPath lists one repository's branches. It carries no query of its
	// own, so pagination opens one.
	branchesPath = "/repos/%s/%s/branches"

	// pageSize is GitHub's maximum; a short page means the last one.
	pageSize        = 100
	paginationQuery = "%s%sper_page=%d&page=%d"

	querySeparator = "&"
	queryStart     = "?"

	// Owner types the identity endpoint reports.
	ownerTypeOrganization = "Organization"
	ownerTypeUser         = "User"

	// cacheSchemaVersion invalidates a cache whose shape predates this build.
	cacheSchemaVersion = 3

	// Token discovery: gh owns keychain and hosts.yml resolution, so ask it
	// first and fall back to the environment.
	ghBin       = "gh"
	ghAuthCmd   = "auth"
	ghTokenCmd  = "token"
	tokenEnvVar = "GITHUB_TOKEN"

	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	acceptHeader        = "Accept"
	acceptGitHubJSON    = "application/vnd.github+json"

	getMethod = "GET"

	repoIDSeparator = "/"

	cacheDirMode  = 0o755
	cacheFileMode = 0o644
)
