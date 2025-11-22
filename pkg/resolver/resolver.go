package resolver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/parser"
)

// ResolverResult contains information about a resolved module
type ResolverResult struct {
	ModulePath      string
	ActualURL       string
	HostingProvider string
	Status          ModuleStatus
	Details         string
	IsRedirect      bool
}

// ModuleStatus represents the status of a resolved module
type ModuleStatus string

const (
	StatusActive      ModuleStatus = "active"
	StatusUnknown     ModuleStatus = "unknown"
	StatusNotFound    ModuleStatus = "not_found"
	StatusRedirect    ModuleStatus = "redirect"
	StatusUnavailable ModuleStatus = "unavailable"
)

// Resolver handles resolution of non-GitHub Go modules
type Resolver struct {
	httpClient *http.Client
	timeout    time.Duration
}

// NewResolver creates a new module resolver
func NewResolver(timeout time.Duration) *Resolver {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &Resolver{
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects automatically - we want to handle them
				return http.ErrUseLastResponse
			},
		},
		timeout: timeout,
	}
}

// ResolveModule attempts to resolve a non-GitHub module and determine its status
func (r *Resolver) ResolveModule(ctx context.Context, modulePath string) *ResolverResult {
	result := &ResolverResult{
		ModulePath: modulePath,
		Status:     StatusUnknown,
	}

	moduleInfo := parser.ParseModulePath(modulePath)
	if !moduleInfo.IsValid {
		result.Status = StatusNotFound
		result.Details = "Invalid module path"
		return result
	}

	result.HostingProvider = moduleInfo.Host

	// Try different resolution strategies
	if resolved := r.tryGoModuleProxy(ctx, modulePath); resolved != nil {
		return resolved
	}

	if resolved := r.tryVanityURL(ctx, modulePath); resolved != nil {
		return resolved
	}

	if resolved := r.tryWellKnownPatterns(modulePath, moduleInfo); resolved != nil {
		return resolved
	}

	// Default to unknown if all resolution attempts fail
	result.Details = "Could not resolve module source"
	return result
}

// tryGoModuleProxy attempts to resolve the module using the Go module proxy
func (r *Resolver) tryGoModuleProxy(ctx context.Context, modulePath string) *ResolverResult {
	// Try the default Go proxy
	proxyURL := fmt.Sprintf("https://proxy.golang.org/%s/@v/list", url.QueryEscape(modulePath))

	req, err := http.NewRequestWithContext(ctx, "GET", proxyURL, nil)
	if err != nil {
		return nil
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	result := &ResolverResult{
		ModulePath:      modulePath,
		ActualURL:       proxyURL,
		HostingProvider: "Go Module Proxy",
		Status:          StatusUnknown,
	}

	switch resp.StatusCode {
	case http.StatusOK:
		result.Status = StatusActive
		result.Details = "Available in Go module proxy"
	case http.StatusNotFound, http.StatusGone:
		result.Status = StatusNotFound
		result.Details = "Not found in Go module proxy"
	default:
		result.Status = StatusUnavailable
		result.Details = fmt.Sprintf("Go module proxy returned status %d", resp.StatusCode)
	}

	return result
}

// tryVanityURL attempts to resolve vanity URLs by fetching the module's import meta tags
func (r *Resolver) tryVanityURL(ctx context.Context, modulePath string) *ResolverResult {
	// Try HTTPS first, then HTTP
	schemes := []string{"https", "http"}

	for _, scheme := range schemes {
		url := fmt.Sprintf("%s://%s?go-get=1", scheme, modulePath)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		resp, err := r.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		result := &ResolverResult{
			ModulePath:      modulePath,
			ActualURL:       url,
			HostingProvider: "Vanity URL",
			Status:          StatusUnknown,
		}

		switch resp.StatusCode {
		case http.StatusOK:
			result.Status = StatusActive
			result.Details = fmt.Sprintf("Vanity URL accessible via %s", scheme)
			return result
		case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
			if location := resp.Header.Get("Location"); location != "" {
				result.Status = StatusRedirect
				result.ActualURL = location
				result.Details = fmt.Sprintf("Redirects to %s", location)
				return result
			}
		case http.StatusNotFound, http.StatusGone:
			result.Status = StatusNotFound
			result.Details = "Vanity URL not found"
			return result
		}
	}

	return nil
}

// tryWellKnownPatterns attempts to resolve modules using well-known patterns
func (r *Resolver) tryWellKnownPatterns(modulePath string, moduleInfo *parser.ModuleInfo) *ResolverResult {
	result := &ResolverResult{
		ModulePath:      modulePath,
		HostingProvider: moduleInfo.Host,
		Status:          StatusUnknown,
	}

	// Handle well-known Go module patterns
	switch {
	case strings.HasPrefix(modulePath, "golang.org/x/"):
		result.HostingProvider = "golang.org"
		result.ActualURL = fmt.Sprintf("https://github.com/golang/%s", strings.TrimPrefix(modulePath, "golang.org/x/"))
		result.Status = StatusActive
		result.Details = "Official Go extended package"

	case strings.HasPrefix(modulePath, "google.golang.org/"):
		result.HostingProvider = "google.golang.org"
		result.Status = StatusActive
		result.Details = "Google Go package"

	case strings.HasPrefix(modulePath, "cloud.google.com/"):
		result.HostingProvider = "cloud.google.com"
		result.Status = StatusActive
		result.Details = "Google Cloud package"

	case strings.HasPrefix(modulePath, "go.uber.org/"):
		result.HostingProvider = "go.uber.org"
		result.Status = StatusActive
		result.Details = "Uber Go package"

	case strings.HasPrefix(modulePath, "gopkg.in/"):
		result.HostingProvider = "gopkg.in"
		result.Status = StatusActive
		result.Details = "gopkg.in package"

	case strings.HasPrefix(modulePath, "k8s.io/"), strings.HasPrefix(modulePath, "sigs.k8s.io/"):
		result.HostingProvider = "k8s.io"
		result.Status = StatusActive
		result.Details = "Kubernetes package"

	default:
		// Try to construct potential GitHub URLs for common patterns
		if moduleInfo.Owner != "" && moduleInfo.Repo != "" {
			result.Details = "Unknown hosting provider, may be self-hosted"
		} else {
			result.Details = "Could not determine hosting provider"
		}
	}

	return result
}

// GetWellKnownModuleInfo returns information about well-known Go modules
func GetWellKnownModuleInfo(modulePath string) *ResolverResult {
	wellKnownModules := map[string]ResolverResult{
		"golang.org": {
			HostingProvider: "golang.org",
			Status:          StatusActive,
			Details:         "Official Go package",
		},
		"google.golang.org": {
			HostingProvider: "google.golang.org",
			Status:          StatusActive,
			Details:         "Google-maintained Go package",
		},
		"cloud.google.com": {
			HostingProvider: "cloud.google.com",
			Status:          StatusActive,
			Details:         "Google Cloud Go package",
		},
		"go.uber.org": {
			HostingProvider: "go.uber.org",
			Status:          StatusActive,
			Details:         "Uber-maintained Go package",
		},
		"gopkg.in": {
			HostingProvider: "gopkg.in",
			Status:          StatusActive,
			Details:         "Versioned package proxy",
		},
		"k8s.io": {
			HostingProvider: "k8s.io",
			Status:          StatusActive,
			Details:         "Kubernetes package",
		},
		"sigs.k8s.io": {
			HostingProvider: "sigs.k8s.io",
			Status:          StatusActive,
			Details:         "Kubernetes SIG package",
		},
	}

	for prefix, info := range wellKnownModules {
		if strings.HasPrefix(modulePath, prefix) {
			result := info
			result.ModulePath = modulePath
			return &result
		}
	}

	return nil
}
