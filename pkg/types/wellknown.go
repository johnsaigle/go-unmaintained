package types

// WellKnownModule represents information about a well-known Go module.
type WellKnownModule struct {
	Prefix          string
	Trusted         bool
	HostingProvider string
	StatusMessage   string
	MapsToGitHub    bool
	GitHubOwner     string
	GitHubRepoPath  string // Template for constructing GitHub repo path from module path
}

// WellKnownModules is the canonical registry of well-known Go modules.
// This centralizes information that was previously duplicated across parser,
// resolver, and analyzer packages.
var WellKnownModules = []WellKnownModule{
	{
		Prefix:          "golang.org/x/",
		Trusted:         true,
		HostingProvider: "golang.org",
		StatusMessage:   "Official Go extended package",
		MapsToGitHub:    true,
		GitHubOwner:     "golang",
		GitHubRepoPath:  "x/{{.Repo}}",
	},
	{
		Prefix:          "google.golang.org/",
		Trusted:         true,
		HostingProvider: "google.golang.org",
		StatusMessage:   "Google-maintained Go package",
	},
	{
		Prefix:          "cloud.google.com/",
		Trusted:         true,
		HostingProvider: "cloud.google.com",
		StatusMessage:   "Google Cloud Go package",
	},
	{
		Prefix:          "go.uber.org/",
		Trusted:         true,
		HostingProvider: "go.uber.org",
		StatusMessage:   "Uber-maintained Go package",
	},
	{
		Prefix:          "gopkg.in/",
		Trusted:         false,
		HostingProvider: "gopkg.in",
		StatusMessage:   "Versioned package proxy",
	},
	{
		Prefix:          "k8s.io/",
		Trusted:         true,
		HostingProvider: "k8s.io",
		StatusMessage:   "Kubernetes package",
	},
	{
		Prefix:          "sigs.k8s.io/",
		Trusted:         true,
		HostingProvider: "sigs.k8s.io",
		StatusMessage:   "Kubernetes SIG package",
	},
	{
		Prefix:          "go.opentelemetry.io/",
		Trusted:         false,
		HostingProvider: "go.opentelemetry.io",
		StatusMessage:   "OpenTelemetry Go package",
	},
}

// IsWellKnownModule checks if a module path matches any well-known module prefix.
func IsWellKnownModule(modulePath string) bool {
	for _, m := range WellKnownModules {
		if len(modulePath) >= len(m.Prefix) && modulePath[:len(m.Prefix)] == m.Prefix {
			return true
		}
	}
	return false
}

// IsTrustedModule checks if a module path matches any trusted module prefix.
func IsTrustedModule(modulePath string) bool {
	for _, m := range WellKnownModules {
		if m.Trusted && len(modulePath) >= len(m.Prefix) && modulePath[:len(m.Prefix)] == m.Prefix {
			return true
		}
	}
	return false
}

// GetWellKnownModule returns the WellKnownModule for a given path, or nil if not found.
func GetWellKnownModule(modulePath string) *WellKnownModule {
	for _, m := range WellKnownModules {
		if len(modulePath) >= len(m.Prefix) && modulePath[:len(m.Prefix)] == m.Prefix {
			return &m
		}
	}
	return nil
}

// GetGitHubMapping returns the GitHub owner and repo for modules that map to GitHub.
// Returns ("", "", false) if the module doesn't have a GitHub mapping.
func GetGitHubMapping(modulePath string) (owner, repo string, ok bool) {
	m := GetWellKnownModule(modulePath)
	if m == nil || !m.MapsToGitHub {
		return "", "", false
	}

	// For golang.org/x/ modules, extract the repo name
	if m.Prefix == "golang.org/x/" {
		repoName := modulePath[len(m.Prefix):]
		// Handle sub-packages like golang.org/x/crypto/ssh
		if idx := findFirstSlash(repoName); idx != -1 {
			repoName = repoName[:idx]
		}
		return m.GitHubOwner, repoName, true
	}

	return "", "", false
}

// GetTrustedStatus returns the status message for a trusted module.
// Returns ("", false) if the module is not trusted.
func GetTrustedStatus(modulePath string) (string, bool) {
	m := GetWellKnownModule(modulePath)
	if m == nil || !m.Trusted {
		return "", false
	}
	return m.StatusMessage, true
}

// findFirstSlash finds the index of the first '/' in a string, or -1 if not found.
func findFirstSlash(s string) int {
	for i, c := range s {
		if c == '/' {
			return i
		}
	}
	return -1
}
