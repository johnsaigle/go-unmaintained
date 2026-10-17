package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGitLabProvider_SupportsHost(t *testing.T) {
	gp := NewGitLabProvider()

	if !gp.SupportsHost("gitlab.com") {
		t.Error("GitLab provider should support gitlab.com")
	}
	if gp.SupportsHost("github.com") {
		t.Error("GitLab provider should not support github.com")
	}
	if gp.SupportsHost("bitbucket.org") {
		t.Error("GitLab provider should not support bitbucket.org")
	}
}

func TestGitLabProvider_GetName(t *testing.T) {
	gp := NewGitLabProvider()
	if gp.GetName() != "GitLab" {
		t.Errorf("GetName() = %q, want %q", gp.GetName(), "GitLab")
	}
}

func testGitLabSuccess(t *testing.T) (string, *GitLabProvider) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.EscapedPath()
		// The literal "%2F" inside a t.Errorf format string would make either
		// vet or staticcheck treat the F as a directive that consumes an arg,
		// so the assertion is split into non-Printf statements.
		if !strings.Contains(raw, "%2F") {
			t.Errorf("GitLab project path %q must contain %%2F between owner and repo", raw)
			return
		}
		if strings.Contains(raw, "//") {
			t.Errorf("GitLab project path %q must not contain an unescaped /", raw)
		}
		project := GitLabProject{
			ID:             123,
			Name:           "testproject",
			Description:    "A test project",
			WebURL:         "https://gitlab.com/org/testproject",
			DefaultBranch:  "main",
			Archived:       false,
			CreatedAt:      now.Add(-365 * 24 * time.Hour),
			LastActivityAt: now.Add(-5 * 24 * time.Hour),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(project); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	return server.URL, newGitLabProvider(server.URL)
}

func TestGitLabProvider_GetRepositoryInfo_Success(t *testing.T) {
	_, gp := testGitLabSuccess(t)

	ctx := context.Background()
	info, err := gp.GetRepositoryInfo(ctx, "org", "testproject")
	if err != nil {
		t.Fatalf("GetRepositoryInfo() error: %v", err)
	}
	if !info.Exists {
		t.Error("expected Exists=true")
	}
	if info.IsArchived {
		t.Error("expected IsArchived=false")
	}
	if info.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want main", info.DefaultBranch)
	}
	if info.LastCommitAt == nil {
		t.Error("expected LastCommitAt to be set")
	}
	if info.Description != "A test project" {
		t.Errorf("Description = %q, want 'A test project'", info.Description)
	}
}

func TestGitLabProvider_GetRepositoryInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gp := newGitLabProvider(server.URL)
	info, err := gp.GetRepositoryInfo(context.Background(), "org", "missing")
	if err != nil {
		t.Fatalf("expected no error for 404, got: %v", err)
	}
	if info == nil || info.Exists {
		t.Errorf("expected Exists=false for 404, got %+v", info)
	}
}

func TestGitLabProvider_GetRepositoryInfo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	gp := newGitLabProvider(server.URL)
	_, err := gp.GetRepositoryInfo(context.Background(), "org", "x")
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestBitbucketProvider_SupportsHost(t *testing.T) {
	bp := NewBitbucketProvider()

	if !bp.SupportsHost("bitbucket.org") {
		t.Error("Bitbucket provider should support bitbucket.org")
	}
	if bp.SupportsHost("github.com") {
		t.Error("Bitbucket provider should not support github.com")
	}
	if bp.SupportsHost("gitlab.com") {
		t.Error("Bitbucket provider should not support gitlab.com")
	}
}

func TestBitbucketProvider_GetName(t *testing.T) {
	bp := NewBitbucketProvider()
	if bp.GetName() != "Bitbucket" {
		t.Errorf("GetName() = %q, want %q", bp.GetName(), "Bitbucket")
	}
}

func TestBitbucketProvider_GetRepositoryInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/o/r" {
			t.Errorf("unexpected Bitbucket path %q, want /repositories/o/r", r.URL.Path)
		}
		resp := BitbucketRepository{
			CreatedOn:   time.Unix(1600000000, 0).UTC(),
			UpdatedOn:   time.Unix(1700000000, 0).UTC(),
			Name:        "r",
			FullName:    "o/r",
			Description: "A test repo",
		}
		resp.Links.HTML.Href = "https://bitbucket.org/o/r"
		resp.MainBranch.Name = "main"
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	bp := newBitbucketProvider(server.URL)
	info, err := bp.GetRepositoryInfo(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("GetRepositoryInfo() error: %v", err)
	}
	if !info.Exists {
		t.Error("expected Exists=true")
	}
	if info.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want main", info.DefaultBranch)
	}
	if info.Description != "A test repo" {
		t.Errorf("Description = %q, want 'A test repo'", info.Description)
	}
	if info.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestBitbucketProvider_GetRepositoryInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bp := newBitbucketProvider(server.URL)
	info, err := bp.GetRepositoryInfo(context.Background(), "o", "missing")
	if err != nil {
		t.Fatalf("expected no error for 404, got: %v", err)
	}
	if info == nil || info.Exists {
		t.Errorf("expected Exists=false for 404, got %+v", info)
	}
}

func TestBitbucketProvider_GetRepositoryInfo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	bp := newBitbucketProvider(server.URL)
	_, err := bp.GetRepositoryInfo(context.Background(), "o", "x")
	if err == nil {
		t.Error("expected error for non-404, non-200 response")
	}
}

func TestMultiProvider_GetRepositoryInfo_UnsupportedHost(t *testing.T) {
	mp := NewMultiProvider()
	ctx := context.Background()

	_, err := mp.GetRepositoryInfo(ctx, "unknown-host.com", "org", "repo")
	if err == nil {
		t.Error("expected error for unsupported host")
	}
}

func TestGetProviderForHost(t *testing.T) {
	tests := []struct {
		host     string
		wantName string
		wantNil  bool
	}{
		{"gitlab.com", "GitLab", false},
		{"bitbucket.org", "Bitbucket", false},
		{"github.com", "", true},
		{"unknown.com", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			p := GetProviderForHost(tt.host)
			if tt.wantNil {
				if p != nil {
					t.Errorf("expected nil for host %q, got %v", tt.host, p.GetName())
				}
				return
			}
			if p == nil {
				t.Fatalf("expected provider for host %q, got nil", tt.host)
			}
			if p.GetName() != tt.wantName {
				t.Errorf("GetName() = %q, want %q", p.GetName(), tt.wantName)
			}
		})
	}
}

func TestNewMultiProvider(t *testing.T) {
	mp := NewMultiProvider()
	if mp == nil {
		t.Fatal("NewMultiProvider() returned nil")
	}
	if len(mp.providers) != 2 {
		t.Errorf("providers count = %d, want 2", len(mp.providers))
	}
}
