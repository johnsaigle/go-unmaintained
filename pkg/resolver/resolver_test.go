package resolver

import (
	"testing"

	"github.com/johnsaigle/go-unmaintained/pkg/parser"
)

func TestGetWellKnownModuleInfo(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		wantNil    bool
		wantStatus ModuleStatus
		wantHost   string
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
