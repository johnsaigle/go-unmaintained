package types

import (
	"testing"
)

func TestIsWellKnownModule(t *testing.T) {
	tests := []struct {
		name string
		mod  string
		want bool
	}{
		{"golang.org/x/net", "golang.org/x/net", true},
		{"golang.org/x/crypto", "golang.org/x/crypto", true},
		{"golang.org/x/sys", "golang.org/x/sys", true},
		{"golang.org/x/text", "golang.org/x/text", true},
		{"golang.org/x/tools", "golang.org/x/tools", true},
		{"golang.org/x/x", "golang.org/x/x", true},
		{"golang.org itself is not a prefix match", "golang.org", false},
		{"golang.org/x (prefix without trailing path) does not match x/ entries exactly", "golang.org/x/", true},
		{"golang.org/x followed by slash matches", "golang.org/x/", true},
		{"golang.org/x followed by path matches", "golang.org/x/net/html", true},
		{"google.golang.org/protobuf", "google.golang.org/protobuf", true},
		{"cloud.google.com/go", "cloud.google.com/go", true},
		{"go.uber.org/atomic", "go.uber.org/atomic", true},
		{"gopkg.in/yaml.v3", "gopkg.in/yaml.v3", true},
		{"k8s.io/api", "k8s.io/api", true},
		{"sigs.k8s.io/controller-runtime", "sigs.k8s.io/controller-runtime", true},
		{"go.opentelemetry.io/otel", "go.opentelemetry.io/otel", true},
		{"unrelated host", "example.com/foo", false},
		{"empty string", "", false},
		{"prefix shorter than module", "gopkg.in", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWellKnownModule(tt.mod); got != tt.want {
				t.Errorf("IsWellKnownModule(%q) = %v, want %v", tt.mod, got, tt.want)
			}
		})
	}
}

func TestIsTrustedModule(t *testing.T) {
	tests := []struct {
		name string
		mod  string
		want bool
	}{
		{"golang.org/x/net trusted", "golang.org/x/net", true},
		{"golang.org/x/crypto/ssh trusted", "golang.org/x/crypto/ssh", true},
		{"google.golang.org/protobuf trusted", "google.golang.org/protobuf", true},
		{"go.uber.org/atomic trusted", "go.uber.org/atomic", true},
		{"k8s.io/api trusted", "k8s.io/api", true},
		{"gopkg.in/yaml.v3 not trusted", "gopkg.in/yaml.v3", false},
		{"go.opentelemetry.io/otel not trusted", "go.opentelemetry.io/otel", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTrustedModule(tt.mod); got != tt.want {
				t.Errorf("IsTrustedModule(%q) = %v, want %v", tt.mod, got, tt.want)
			}
		})
	}
}

func TestGetWellKnownModule(t *testing.T) {
	t.Run("golang.org/x/net returns the x/ entry", func(t *testing.T) {
		m := GetWellKnownModule("golang.org/x/net")
		if m == nil {
			t.Fatal("expected non-nil result")
		}
		if m.Prefix != "golang.org/x/" {
			t.Errorf("Prefix = %q, want golang.org/x/", m.Prefix)
		}
		if m.GitHubOwner != "golang" {
			t.Errorf("GitHubOwner = %q, want golang", m.GitHubOwner)
		}
	})
	t.Run("gopkg.in is trusted=false", func(t *testing.T) {
		m := GetWellKnownModule("gopkg.in/yaml.v3")
		if m == nil {
			t.Fatal("expected non-nil result")
		}
		if m.Trusted {
			t.Errorf("Trusted = true, want false for gopkg.in")
		}
	})
	t.Run("no match returns nil", func(t *testing.T) {
		if m := GetWellKnownModule("example.com/foo"); m != nil {
			t.Errorf("expected nil, got %v", m)
		}
	})
}

func TestGetGitHubMapping(t *testing.T) {
	t.Run("golang.org/x/net maps to golang/net", func(t *testing.T) {
		owner, repo, ok := GetGitHubMapping("golang.org/x/net")
		if !ok {
			t.Fatal("expected mapping to be found")
		}
		if owner != "golang" {
			t.Errorf("owner = %q, want golang", owner)
		}
		if repo != "net" {
			t.Errorf("repo = %q, want net", repo)
		}
	})
	t.Run("golang.org/x/crypto/ssh strips subpackage to crypto", func(t *testing.T) {
		owner, repo, ok := GetGitHubMapping("golang.org/x/crypto/ssh")
		if !ok {
			t.Fatal("expected mapping to be found")
		}
		if owner != "golang" {
			t.Errorf("owner = %q, want golang", owner)
		}
		if repo != "crypto" {
			t.Errorf("repo = %q, want crypto (sub-package stripped)", repo)
		}
	})
	t.Run("google.golang.org has no GitHub mapping", func(t *testing.T) {
		if _, _, ok := GetGitHubMapping("google.golang.org/protobuf"); ok {
			t.Error("google.golang.org should not map to GitHub")
		}
	})
	t.Run("example.com has no mapping", func(t *testing.T) {
		if _, _, ok := GetGitHubMapping("example.com/foo"); ok {
			t.Error("example.com should not map to GitHub")
		}
	})
}

func TestGetTrustedStatus(t *testing.T) {
	t.Run("trusted module returns status message", func(t *testing.T) {
		msg, ok := GetTrustedStatus("golang.org/x/net")
		if !ok {
			t.Fatal("expected golang.org/x/net to be trusted")
		}
		if msg == "" {
			t.Error("expected non-empty status message")
		}
	})
	t.Run("untrusted module returns false", func(t *testing.T) {
		_, ok := GetTrustedStatus("gopkg.in/yaml.v3")
		if ok {
			t.Error("gopkg.in should not be trusted")
		}
	})
	t.Run("unknown module returns false", func(t *testing.T) {
		_, ok := GetTrustedStatus("example.com/foo")
		if ok {
			t.Error("unknown module should not be trusted")
		}
	})
}

func TestFindFirstSlash(t *testing.T) {
	if got := findFirstSlash("crypto/ssh"); got != 6 {
		t.Errorf("findFirstSlash(crypto/ssh) = %d, want 6", got)
	}
	if got := findFirstSlash("no-slash"); got != -1 {
		t.Errorf("findFirstSlash(no-slash) = %d, want -1", got)
	}
	if got := findFirstSlash(""); got != -1 {
		t.Errorf("findFirstSlash(\"\") = %d, want -1", got)
	}
	if got := findFirstSlash("/leading"); got != 0 {
		t.Errorf("findFirstSlash(/leading) = %d, want 0", got)
	}
}