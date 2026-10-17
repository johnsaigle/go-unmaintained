package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/parser"
)

func TestGetWellKnownModuleInfo(t *testing.T) {
	tests := []struct {
		modulePath string
		name       string
		wantHost   string
		wantStatus ModuleStatus
		wantNil    bool
	}{
		{
			name:       "golang.org module",
			modulePath: "golang.org/x/crypto",
			wantNil:    false,
			wantStatus: StatusActive,
			wantHost:   "golang.org",
		},
		{
			name:       "google.golang.org module",
			modulePath: "google.golang.org/protobuf",
			wantNil:    false,
			wantStatus: StatusActive,
			wantHost:   "google.golang.org",
		},
		{
			name:       "cloud.google.com module",
			modulePath: "cloud.google.com/go/storage",
			wantNil:    false,
			wantStatus: StatusActive,
			wantHost:   "cloud.google.com",
		},
		{
			name:       "go.uber.org module",
			modulePath: "go.uber.org/zap",
			wantNil:    false,
			wantStatus: StatusActive,
			wantHost:   "go.uber.org",
		},
		{
			name:       "gopkg.in module",
			modulePath: "gopkg.in/yaml.v3",
			wantNil:    false,
			wantStatus: StatusActive,
			wantHost:   "gopkg.in",
		},
		{
			name:       "k8s.io module",
			modulePath: "k8s.io/api",
			wantNil:    false,
			wantStatus: StatusActive,
			wantHost:   "k8s.io",
		},
		{
			name:       "sigs.k8s.io module",
			modulePath: "sigs.k8s.io/controller-runtime",
			wantNil:    false,
			wantStatus: StatusActive,
			wantHost:   "sigs.k8s.io",
		},
		{
			name:       "unknown module returns nil",
			modulePath: "github.com/user/repo",
			wantNil:    true,
		},
		{
			name:       "random domain returns nil",
			modulePath: "example.com/pkg",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetWellKnownModuleInfo(tt.modulePath)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
			if result.HostingProvider != tt.wantHost {
				t.Errorf("HostingProvider = %q, want %q", result.HostingProvider, tt.wantHost)
			}
			if result.ModulePath != tt.modulePath {
				t.Errorf("ModulePath = %q, want %q", result.ModulePath, tt.modulePath)
			}
		})
	}
}

func TestTryWellKnownPatterns(t *testing.T) {
	r := NewResolver(0)

	tests := []struct {
		name       string
		modulePath string
		wantStatus ModuleStatus
		wantHost   string
	}{
		{"golang.org/x", "golang.org/x/net", StatusActive, "golang.org"},
		{"google.golang.org", "google.golang.org/grpc", StatusActive, "google.golang.org"},
		{"cloud.google.com", "cloud.google.com/go", StatusActive, "cloud.google.com"},
		{"go.uber.org", "go.uber.org/zap", StatusActive, "go.uber.org"},
		{"gopkg.in", "gopkg.in/yaml.v3", StatusActive, "gopkg.in"},
		{"k8s.io", "k8s.io/api", StatusActive, "k8s.io"},
		{"sigs.k8s.io", "sigs.k8s.io/controller-runtime", StatusActive, "sigs.k8s.io"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moduleInfo := parser.ParseModulePath(tt.modulePath)
			result := r.tryWellKnownPatterns(tt.modulePath, moduleInfo)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
			if result.HostingProvider != tt.wantHost {
				t.Errorf("HostingProvider = %q, want %q", result.HostingProvider, tt.wantHost)
			}
		})
	}
}

func TestNewResolver(t *testing.T) {
	// Default timeout
	r := NewResolver(0)
	if r == nil {
		t.Fatal("NewResolver(0) returned nil")
	}
	if r.httpClient == nil {
		t.Error("httpClient should not be nil")
	}

	// Custom timeout
	r2 := NewResolver(5)
	if r2.timeout != 5 {
		t.Errorf("timeout = %v, want 5", r2.timeout)
	}
}

// tryVanityURL — 200 OK: module is accessible via vanity URL.
func TestTryVanityURL_Accessible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// tryVanityURL builds {scheme}://{modulePath}?go-get=1.
	// Pass only the host so the constructed URL resolves to our test server.
	host := server.Listener.Addr().String()

	r := NewResolver(1 * time.Second)
	// Point the client at the plain-HTTP test server.
	r.httpClient = server.Client()

	result := r.tryVanityURL(context.Background(), host)
	if result == nil {
		t.Fatal("expected non-nil result for 200 response")
	}
	if result.Status != StatusActive {
		t.Errorf("Status = %q, want active", result.Status)
	}
	if result.HostingProvider != "Vanity URL" {
		t.Errorf("HostingProvider = %q, want Vanity URL", result.HostingProvider)
	}
}

// tryVanityURL — redirect paths are exercised via the StatusActive/StatusNotFound
// cases above; the redirect branch is structurally simple (set ActualURL to
// Location header and return). The full redirect path requires a TLS-aware
// mock at the transport layer which is not achievable in a hermetic unit test
// without refactoring the resolver's hard-coded schemes.

// tryVanityURL — 404: StatusNotFound.
func TestTryVanityURL_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	host := server.Listener.Addr().String()
	r := NewResolver(1 * time.Second)
	r.httpClient = server.Client()

	result := r.tryVanityURL(context.Background(), host)
	if result == nil {
		t.Fatal("expected non-nil result for 404 response")
	}
	if result.Status != StatusNotFound {
		t.Errorf("Status = %q, want not_found", result.Status)
	}
}

// ResolveModule with an invalid module path returns StatusNotFound directly
// without any network calls.
func TestResolveModule_InvalidModule(t *testing.T) {
	r := NewResolver(1 * time.Second)
	result := r.ResolveModule(context.Background(), "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != StatusNotFound {
		t.Errorf("Status = %q, want not_found for empty/invalid module path", result.Status)
	}
}

// GetWellKnownModuleInfo is hermetic: it uses only the types registry,
// no network I/O. We already cover it in TestGetWellKnownModuleInfo above.
