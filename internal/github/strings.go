package github

// GitHub API endpoints, query parameters, and the shell-outs qrouton uses to
// find a token. Endpoint paths are built from these rather than spelled out at
// each call, so githubAPIBase stays the only thing a test has to swap.

const (
	// apiBaseDefault is overridable in tests via githubAPIBase.
	apiBaseDefault = "https://api.github.com"

	usersPath = "/users/"
	orgsPath  = "/orgs/"
	reposPath = "/repos"
	userPath  = "/user"

	// userReposQuery lists the authenticated user's own repositories, including
	// private ones — the /users/<login>/repos endpoint exposes only public.
	userReposQuery = "/user/repos?affiliation=owner&visibility=all"
	orgReposQuery  = "?type=all"
	otherUserQuery = "?type=owner"

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
