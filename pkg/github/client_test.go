package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockGitHub serves the REST endpoints the client calls:
//
//	GET /repos/{owner}/{repo}        -> 200 (repo) | 404 | 401 | 403 (rate-limit)
//	GET /repos/{owner}/{repo}/commits -> 200 with a single commit JSON
//	GET /repos/{owner}/{repo}/tags    -> 200 with a JSON array of tags
func newMockGitHub(t *testing.T, behavior func(r *http.Request) (int, string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, body := behavior(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
}

// GetRepositoryInfo — happy path with a present repo.
func TestGetRepositoryInfo_Found(t *testing.T) {
	server := newMockGitHub(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/repos/o/r":
			return 200, `{"archived":false,"description":"d","default_branch":"main","created_at":"2020-01-01T00:00:00Z","updated_at":"2024-06-01T00:00:00Z","html_url":"https://github.com/o/r"}`
		case "/repos/o/r/commits":
			return 200, `[{"commit":{"committer":{"date":"2024-06-01T00:00:00Z"}}}]`
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			return 404, ""
		}
	})
	defer server.Close()

	c := NewClientForTest(server.URL)
	info, err := c.GetRepositoryInfo(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("GetRepositoryInfo() error: %v", err)
	}
	if !info.Exists {
		t.Error("expected Exists=true")
	}
	if info.IsArchived {
		t.Error("expected not archived")
	}
	if info.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want main", info.DefaultBranch)
	}
	if info.LastCommitAt == nil {
		t.Error("expected LastCommitAt to be set")
	}
}

func TestGetRepositoryInfo_Archived(t *testing.T) {
	server := newMockGitHub(t, func(r *http.Request) (int, string) {
		if r.URL.Path == "/repos/o/r/archived" || (r.URL.Path == "/repos/o/r" && r.Method == "GET") {
			return 200, `{"archived":true,"default_branch":"main","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-02-01T00:00:00Z","html_url":"https://github.com/o/r"}`
		}
		return 200, `[{"commit":{"committer":{"date":"2020-02-01T00:00:00Z"}}}]`
	})
	defer server.Close()

	c := NewClientForTest(server.URL)
	info, err := c.GetRepositoryInfo(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("GetRepositoryInfo() error: %v", err)
	}
	if !info.IsArchived {
		t.Error("expected IsArchived=true")
	}
}

// A 404 must surface as Exists:false (no error) so the caller can mark the
// repo as not-found rather than propagating an error.
func TestGetRepositoryInfo_NotFound(t *testing.T) {
	server := newMockGitHub(t, func(r *http.Request) (int, string) {
		return 404, `{"message":"Not Found"}`
	})
	defer server.Close()

	c := NewClientForTest(server.URL)
	info, err := c.GetRepositoryInfo(context.Background(), "o", "missing")
	if err != nil {
		t.Fatalf("expected no error for 404, got: %v", err)
	}
	if info == nil || info.Exists {
		t.Errorf("expected Exists=false for 404, got %+v", info)
	}
}

// A 401 must be an error (bad token), not swallowed.
func TestGetRepositoryInfo_Unauthorized(t *testing.T) {
	server := newMockGitHub(t, func(r *http.Request) (int, string) {
		return 401, `{"message":"Bad credentials"}`
	})
	defer server.Close()

	c := NewClientForTest(server.URL)
	_, err := c.GetRepositoryInfo(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected an error for 401")
	}
}

// A 403 rate-limit (X-RateLimit-Remaining: 0) must be an error.
func TestGetRepositoryInfo_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(403)
		fmt.Fprint(w, `{"message":"rate limit exceeded"}`)
	}))
	defer server.Close()

	c := NewClientForTest(server.URL)
	_, err := c.GetRepositoryInfo(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected an error for rate-limited 403")
	}
	if s := err.Error(); s == "" {
		t.Error("expected a non-empty error message")
	}
}

// GetLatestVersion — when a semantic tag is present, the newest valid tag wins.
func TestGetLatestVersion_SemanticTag(t *testing.T) {
	server := newMockGitHub(t, func(r *http.Request) (int, string) {
		if r.URL.Path == "/repos/o/r/tags" {
			return 200, `[{"name":"v9.8.7"},{"name":"v1.2.3"},{"name":"notatag"}]`
		}
		return 404, ""
	})
	defer server.Close()

	c := NewClientForTest(server.URL)
	latest, err := c.GetLatestVersion(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("GetLatestVersion() error: %v", err)
	}
	if latest != "v9.8.7" {
		t.Errorf("GetLatestVersion() = %q, want v9.8.7", latest)
	}
}

// GetLatestVersion — when no semantic tag is present, the first tag is returned.
// This pins the documented "fallback to newest-listed tag" behavior.
func TestGetLatestVersion_FallbackToNonSemanticTag(t *testing.T) {
	server := newMockGitHub(t, func(r *http.Request) (int, string) {
		if r.URL.Path == "/repos/o/r/tags" {
			return 200, `[{"name":"v4.0.0-alpha.some"},{"name":"feature-branch"}]`
		}
		return 404, ""
	})
	defer server.Close()

	c := NewClientForTest(server.URL)
	latest, err := c.GetLatestVersion(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("GetLatestVersion() error: %v", err)
	}
	// v4.0.0-alpha.some is a valid semver with a pre-release, "feature-branch"
	// is not. The newest listed valid one is returned.
	if latest != "v4.0.0-alpha.some" {
		t.Errorf("GetLatestVersion() = %q, want v4.0.0-alpha.some", latest)
	}
}

// GetLatestVersion — no tags at all must be an error.
func TestGetLatestVersion_NoTags(t *testing.T) {
	server := newMockGitHub(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/repos/o/r/tags":
			return 200, `[]`
		default:
			return 404, ""
		}
	})
	defer server.Close()

	c := NewClientForTest(server.URL)
	_, err := c.GetLatestVersion(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected an error when no tags exist")
	}
}

// ValidateToken — 200 passes, 401 fails, 403-with-rate-limit fails,
// 404 is tolerated (no change to behavior).
func newMockGitHubWithStatus(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status == 403 {
			w.Header().Set("X-RateLimit-Remaining", "0")
		}
		w.WriteHeader(status)
	}))
}

func TestValidateToken(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"ok", 200, false},
		{"unauthorized", 401, true},
		{"rate-limited-403", 403, true},
		{"not-found-tolerated", 404, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newMockGitHubWithStatus(t, tc.status)
			defer server.Close()
			c := NewClientForTest(server.URL)
			err := c.ValidateToken(context.Background())
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateToken() err = %v, want err=%v", err, tc.wantErr)
			}
		})
	}
}

// NewClientForTest — constructible with an arbitrary base URL and no panic.
func TestNewClientForTest(t *testing.T) {
	c := NewClientForTest("http://127.0.0.1:1")
	if c == nil || c.client == nil {
		t.Fatal("NewClientForTest returned a broken client")
	}
	if c.baseURL != "http://127.0.0.1:1" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://127.0.0.1:1")
	}
}
