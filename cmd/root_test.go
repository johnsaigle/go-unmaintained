package cmd

import "testing"

func TestDetermineFormat(t *testing.T) {
	// Save and restore global flags so tests don't interfere with each other
	origJSON := jsonOutput
	origGH := githubActions
	origFormat := outputFormat
	defer func() {
		jsonOutput = origJSON
		githubActions = origGH
		outputFormat = origFormat
	}()

	tests := []struct {
		name         string
		jsonOutput   bool
		ghActions    bool
		outputFormat string
		want         string
	}{
		{"default console", false, false, "console", "console"},
		{"json via outputFormat", false, false, "json", "json"},
		{"github-actions via outputFormat", false, false, "github-actions", "github-actions"},
		{"legacy jsonOutput flag wins", true, false, "console", "json"},
		{"legacy githubActions flag wins", false, true, "console", "github-actions"},
		{"jsonOutput beats githubActions", true, true, "console", "json"},
		{"golangci-lint format passes through", false, false, "golangci-lint", "golangci-lint"},
		{"empty outputFormat returns empty", false, false, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonOutput = tt.jsonOutput
			githubActions = tt.ghActions
			outputFormat = tt.outputFormat
			if got := determineFormat(); got != tt.want {
				t.Errorf("determineFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnalyzeSinglePackage_MalformedInput(t *testing.T) {
	// analyzeSinglePackage validates the package@version format before creating
	// any GitHub client, so these cases need no token.
	origToken := token
	defer func() { token = origToken }()
	token = "dummy-token"

	t.Run("no version separator returns error", func(t *testing.T) {
		err := analyzeSinglePackage("golang.org/x/net")
		if err == nil {
			t.Error("expected error for package without @version")
		}
	})

	t.Run("empty string returns error", func(t *testing.T) {
		err := analyzeSinglePackage("")
		if err == nil {
			t.Error("expected error for empty package string")
		}
	})
}