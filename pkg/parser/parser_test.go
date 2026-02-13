package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseModulePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected ModuleInfo
	}{
		{
			name: "standard GitHub module",
			path: "github.com/user/repo",
			expected: ModuleInfo{
				Host:        "github.com",
				Owner:       "user",
				Repo:        "repo",
				IsGitHub:    true,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "GitHub module with sub-package",
			path: "github.com/user/repo/pkg/sub",
			expected: ModuleInfo{
				Host:        "github.com",
				Owner:       "user",
				Repo:        "repo",
				IsGitHub:    true,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "GitLab module",
			path: "gitlab.com/org/project",
			expected: ModuleInfo{
				Host:        "gitlab.com",
				Owner:       "org",
				Repo:        "project",
				IsGitHub:    false,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "Bitbucket module",
			path: "bitbucket.org/team/lib",
			expected: ModuleInfo{
				Host:        "bitbucket.org",
				Owner:       "team",
				Repo:        "lib",
				IsGitHub:    false,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "golang.org/x module",
			path: "golang.org/x/crypto",
			expected: ModuleInfo{
				Host:        "golang.org",
				Owner:       "x",
				Repo:        "crypto",
				IsGitHub:    false,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "google.golang.org module (2 segments, no owner/repo extraction)",
			path: "google.golang.org/protobuf",
			expected: ModuleInfo{
				Host:        "google.golang.org",
				Owner:       "", // Only 2 path segments, needs >= 3 for owner
				Repo:        "",
				IsGitHub:    false,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "k8s.io module path (k8s.io is not a valid Go module host per module.CheckPath)",
			path: "k8s.io/api/core/v1",
			expected: ModuleInfo{
				Host:        "", // module.CheckPath rejects k8s.io paths
				Owner:       "",
				Repo:        "",
				IsGitHub:    false,
				IsKnownHost: false,
				IsValid:     false, // Not valid per module.CheckPath
			},
		},
		{
			name: "go.uber.org module (2 segments)",
			path: "go.uber.org/zap",
			expected: ModuleInfo{
				Host:        "go.uber.org",
				Owner:       "", // Only 2 segments
				Repo:        "",
				IsGitHub:    false,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "gopkg.in module (2 segments)",
			path: "gopkg.in/yaml.v3",
			expected: ModuleInfo{
				Host:        "gopkg.in",
				Owner:       "", // Only 2 segments
				Repo:        "",
				IsGitHub:    false,
				IsKnownHost: true,
				IsValid:     true,
			},
		},
		{
			name: "unknown host with owner/repo",
			path: "example.com/org/repo",
			expected: ModuleInfo{
				Host:        "example.com",
				Owner:       "org",
				Repo:        "repo",
				IsGitHub:    false,
				IsKnownHost: false,
				IsValid:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseModulePath(tt.path)
			if result.Host != tt.expected.Host {
				t.Errorf("Host = %q, want %q", result.Host, tt.expected.Host)
			}
			if result.Owner != tt.expected.Owner {
				t.Errorf("Owner = %q, want %q", result.Owner, tt.expected.Owner)
			}
			if result.Repo != tt.expected.Repo {
				t.Errorf("Repo = %q, want %q", result.Repo, tt.expected.Repo)
			}
			if result.IsGitHub != tt.expected.IsGitHub {
				t.Errorf("IsGitHub = %v, want %v", result.IsGitHub, tt.expected.IsGitHub)
			}
			if result.IsKnownHost != tt.expected.IsKnownHost {
				t.Errorf("IsKnownHost = %v, want %v", result.IsKnownHost, tt.expected.IsKnownHost)
			}
			if result.IsValid != tt.expected.IsValid {
				t.Errorf("IsValid = %v, want %v", result.IsValid, tt.expected.IsValid)
			}
		})
	}
}

func TestGetGitHubMapping(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantOwner string
		wantRepo  string
		wantOk    bool
	}{
		{
			name:      "golang.org/x/crypto",
			path:      "golang.org/x/crypto",
			wantOwner: "golang",
			wantRepo:  "crypto",
			wantOk:    true,
		},
		{
			name:      "golang.org/x/crypto/ssh sub-package",
			path:      "golang.org/x/crypto/ssh",
			wantOwner: "golang",
			wantRepo:  "crypto",
			wantOk:    true,
		},
		{
			name:      "golang.org/x/text",
			path:      "golang.org/x/text",
			wantOwner: "golang",
			wantRepo:  "text",
			wantOk:    true,
		},
		{
			name:      "non-golang.org path",
			path:      "github.com/user/repo",
			wantOwner: "",
			wantRepo:  "",
			wantOk:    false,
		},
		{
			name:      "google.golang.org is not golang.org/x",
			path:      "google.golang.org/protobuf",
			wantOwner: "",
			wantRepo:  "",
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ok := GetGitHubMapping(tt.path)
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
		})
	}
}

func TestIsTrustedGoModule(t *testing.T) {
	trusted := []string{
		"golang.org/x/crypto",
		"golang.org/x/text/language",
		"google.golang.org/protobuf",
		"google.golang.org/grpc",
		"cloud.google.com/go/storage",
		"k8s.io/api",
		"sigs.k8s.io/controller-runtime",
		"go.uber.org/zap",
	}

	untrusted := []string{
		"github.com/user/repo",
		"gitlab.com/org/project",
		"gopkg.in/yaml.v3",
		"go.opentelemetry.io/otel",
		"example.com/pkg",
	}

	for _, path := range trusted {
		t.Run("trusted:"+path, func(t *testing.T) {
			if !IsTrustedGoModule(path) {
				t.Errorf("IsTrustedGoModule(%q) = false, want true", path)
			}
		})
	}

	for _, path := range untrusted {
		t.Run("untrusted:"+path, func(t *testing.T) {
			if IsTrustedGoModule(path) {
				t.Errorf("IsTrustedGoModule(%q) = true, want false", path)
			}
		})
	}
}

func TestIsValidModulePath(t *testing.T) {
	valid := []string{
		"github.com/user/repo",
		"golang.org/x/crypto",
		"example.com/pkg",
	}

	for _, path := range valid {
		t.Run("valid:"+path, func(t *testing.T) {
			if !IsValidModulePath(path) {
				t.Errorf("IsValidModulePath(%q) = false, want true", path)
			}
		})
	}
}

func TestParseModulePathLegacy(t *testing.T) {
	host, owner, repo, err := ParseModulePathLegacy("github.com/user/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "github.com" || owner != "user" || repo != "repo" {
		t.Errorf("got (%q, %q, %q), want (github.com, user, repo)", host, owner, repo)
	}
}

func TestParseGoMod(t *testing.T) {
	// Create a temp directory with a valid go.mod
	dir := t.TempDir()
	goModContent := `module example.com/myproject

go 1.21

require (
	github.com/user/repo v1.2.3
	golang.org/x/text v0.14.0
)

require (
	github.com/indirect/dep v0.5.0 // indirect
)

replace github.com/user/repo => github.com/fork/repo v1.2.4
`
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	mod, err := ParseGoMod(dir)
	if err != nil {
		t.Fatalf("ParseGoMod() error: %v", err)
	}

	// Check module path
	if mod.Path != "example.com/myproject" {
		t.Errorf("Path = %q, want %q", mod.Path, "example.com/myproject")
	}

	// Check Go version
	if mod.GoVersion != "1.21" {
		t.Errorf("GoVersion = %q, want %q", mod.GoVersion, "1.21")
	}

	// Check dependencies count
	if len(mod.Dependencies) != 3 {
		t.Fatalf("len(Dependencies) = %d, want 3", len(mod.Dependencies))
	}

	// Check direct dependency
	found := false
	for _, dep := range mod.Dependencies {
		if dep.Path == "github.com/user/repo" {
			found = true
			if dep.Version != "v1.2.3" {
				t.Errorf("version = %q, want %q", dep.Version, "v1.2.3")
			}
			if dep.Indirect {
				t.Error("expected direct dependency, got indirect")
			}
			if dep.Replace == nil {
				t.Error("expected replace directive, got nil")
			} else if dep.Replace.NewPath != "github.com/fork/repo" {
				t.Errorf("replace path = %q, want %q", dep.Replace.NewPath, "github.com/fork/repo")
			}
		}
	}
	if !found {
		t.Error("github.com/user/repo not found in dependencies")
	}

	// Check indirect dependency
	for _, dep := range mod.Dependencies {
		if dep.Path == "github.com/indirect/dep" {
			if !dep.Indirect {
				t.Error("expected indirect dependency, got direct")
			}
		}
	}

	// Check replaces
	if len(mod.Replaces) != 1 {
		t.Fatalf("len(Replaces) = %d, want 1", len(mod.Replaces))
	}
}

func TestParseGoMod_MissingFile(t *testing.T) {
	_, err := ParseGoMod(t.TempDir())
	if err == nil {
		t.Error("expected error for missing go.mod, got nil")
	}
}

func TestParseGoMod_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("this is not valid"), 0644)
	if err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	_, err = ParseGoMod(dir)
	if err == nil {
		t.Error("expected error for invalid go.mod, got nil")
	}
}

func TestIsWellKnownGoModule(t *testing.T) {
	wellKnown := []string{
		"golang.org/x/crypto",
		"google.golang.org/protobuf",
		"cloud.google.com/go",
		"go.uber.org/zap",
		"go.opentelemetry.io/otel",
		"gopkg.in/yaml.v3",
		"k8s.io/api",
		"sigs.k8s.io/controller-runtime",
	}

	notWellKnown := []string{
		"github.com/user/repo",
		"gitlab.com/org/project",
		"example.com/pkg",
	}

	for _, path := range wellKnown {
		t.Run("wellknown:"+path, func(t *testing.T) {
			if !isWellKnownGoModule(path) {
				t.Errorf("isWellKnownGoModule(%q) = false, want true", path)
			}
		})
	}

	for _, path := range notWellKnown {
		t.Run("not_wellknown:"+path, func(t *testing.T) {
			if isWellKnownGoModule(path) {
				t.Errorf("isWellKnownGoModule(%q) = true, want false", path)
			}
		})
	}
}
