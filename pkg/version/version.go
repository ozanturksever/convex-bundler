package version

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Detect detects the version using the following priority:
// 1. CLI override (if provided)
// 2. Git tags (if in a git repository)
// 3. package.json version field
// 4. Default "0.0.0"
func Detect(appPath string, cliOverride string) (string, error) {
	// Priority 1: CLI override
	if cliOverride != "" {
		return cliOverride, nil
	}

	// Priority 2: Git tags
	if version, err := detectFromGitTag(appPath); err == nil && version != "" {
		return version, nil
	}

	// Priority 3: package.json
	if version, err := detectFromPackageJSON(appPath); err == nil && version != "" {
		return version, nil
	}

	// Default
	return "0.0.0", nil
}

// detectFromGitTag attempts to get version from the latest git tag
func detectFromGitTag(appPath string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = appPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(output))
	// Remove 'v' prefix if present (e.g., v1.0.0 -> 1.0.0)
	version = strings.TrimPrefix(version, "v")
	return version, nil
}

// detectFromPackageJSON reads version from package.json
func detectFromPackageJSON(appPath string) (string, error) {
	packageJSONPath := filepath.Join(appPath, "package.json")
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return "", err
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", err
	}

	return pkg.Version, nil
}
