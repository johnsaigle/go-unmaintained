package resolver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// RetractionInfo holds information about a retracted version
type RetractionInfo struct {
	IsRetracted bool
	Reason      string
	Ranges      []RetractionRange
}

// RetractionRange represents a single retraction directive
type RetractionRange struct {
	Low    string // Start of range (inclusive)
	High   string // End of range (inclusive), same as Low for single version
	Reason string // Comment explaining the retraction
}

// CheckRetraction checks if a specific version is retracted by fetching the @latest go.mod
func (r *Resolver) CheckRetraction(ctx context.Context, modulePath, version string) (*RetractionInfo, error) {
	info := &RetractionInfo{
		IsRetracted: false,
		Ranges:      []RetractionRange{},
	}

	// Fetch the @latest version's go.mod from the module proxy
	proxyURL := fmt.Sprintf("https://proxy.golang.org/%s/@latest", url.QueryEscape(modulePath))

	req, err := http.NewRequestWithContext(ctx, "GET", proxyURL, nil)
	if err != nil {
		return info, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return info, fmt.Errorf("failed to fetch @latest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Module doesn't exist or proxy error - not an error for us
		return info, nil
	}

	// Read the response to get the latest version
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return info, fmt.Errorf("failed to read @latest response: %w", err)
	}

	// Parse JSON to get the version (simple parsing)
	// Response format: {"Version":"v1.2.3","Time":"2021-01-01T00:00:00Z"}
	versionPattern := regexp.MustCompile(`"Version"\s*:\s*"([^"]+)"`)
	matches := versionPattern.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return info, nil
	}

	latestVersion := matches[1]

	// Now fetch the go.mod file for the latest version
	goModURL := fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.mod",
		url.QueryEscape(modulePath), url.QueryEscape(latestVersion))

	req, err = http.NewRequestWithContext(ctx, "GET", goModURL, nil)
	if err != nil {
		return info, fmt.Errorf("failed to create go.mod request: %w", err)
	}

	resp, err = r.httpClient.Do(req)
	if err != nil {
		return info, fmt.Errorf("failed to fetch go.mod: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info, nil
	}

	// Parse the go.mod file for retract directives
	retractions, err := parseRetractions(resp.Body)
	if err != nil {
		return info, fmt.Errorf("failed to parse retractions: %w", err)
	}

	info.Ranges = retractions

	// Check if the specified version falls within any retraction range
	for _, retract := range retractions {
		if versionInRange(version, retract.Low, retract.High) {
			info.IsRetracted = true
			info.Reason = retract.Reason
			break
		}
	}

	return info, nil
}

// parseRetractions parses retract directives from a go.mod file
func parseRetractions(r io.Reader) ([]RetractionRange, error) {
	var retractions []RetractionRange
	scanner := bufio.NewScanner(r)

	var currentComment string
	var inRetractBlock bool

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Capture comments
		if strings.HasPrefix(trimmed, "//") {
			comment := strings.TrimPrefix(trimmed, "//")
			comment = strings.TrimSpace(comment)
			if currentComment != "" {
				currentComment += " "
			}
			currentComment += comment
			continue
		}

		// Check for retract block start
		if strings.HasPrefix(trimmed, "retract (") {
			inRetractBlock = true
			currentComment = "" // Reset comment for block
			continue
		}

		// Check for retract block end
		if inRetractBlock && trimmed == ")" {
			inRetractBlock = false
			currentComment = ""
			continue
		}

		// Parse retract directive (single line or in block)
		if strings.HasPrefix(trimmed, "retract ") || inRetractBlock {
			retract := parseRetractLine(trimmed, currentComment)
			if retract != nil {
				retractions = append(retractions, *retract)
			}
			currentComment = "" // Reset after use
		}

		// Reset comment if not a comment or retract line
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "retract") && !inRetractBlock {
			currentComment = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return retractions, nil
}

// parseRetractLine parses a single retract line
func parseRetractLine(line, comment string) *RetractionRange {
	line = strings.TrimSpace(line)

	// Remove "retract " prefix if present
	line = strings.TrimPrefix(line, "retract ")
	line = strings.TrimSpace(line)

	// Handle inline comments
	if idx := strings.Index(line, "//"); idx != -1 {
		if comment == "" {
			comment = strings.TrimSpace(line[idx+2:])
		}
		line = strings.TrimSpace(line[:idx])
	}

	// Empty line
	if line == "" {
		return nil
	}

	// Check for range format: [v1.0.0, v1.0.5]
	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		inner := line[1 : len(line)-1]
		parts := strings.Split(inner, ",")
		if len(parts) == 2 {
			low := strings.TrimSpace(parts[0])
			high := strings.TrimSpace(parts[1])
			return &RetractionRange{
				Low:    low,
				High:   high,
				Reason: comment,
			}
		}
	}

	// Single version format: v1.0.0
	version := strings.TrimSpace(line)
	if semver.IsValid(version) {
		return &RetractionRange{
			Low:    version,
			High:   version,
			Reason: comment,
		}
	}

	return nil
}

// versionInRange checks if a version falls within a retraction range
func versionInRange(version, low, high string) bool {
	// Ensure all versions are valid semver
	if !semver.IsValid(version) || !semver.IsValid(low) || !semver.IsValid(high) {
		return false
	}

	// Check if version >= low AND version <= high
	return semver.Compare(version, low) >= 0 && semver.Compare(version, high) <= 0
}
