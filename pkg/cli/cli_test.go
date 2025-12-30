package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ValidArgs(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/path/to/app",
		"--output", "/path/to/output",
		"--backend-binary", "/path/to/backend",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"/path/to/app"}, config.Apps)
	assert.Equal(t, "/path/to/output", config.Output)
	assert.Equal(t, "/path/to/backend", config.BackendBinary)
}

func TestParse_MultipleApps(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app1",
		"--app", "/app2",
		"--output", "/out",
		"--backend-binary", "/backend",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"/app1", "/app2"}, config.Apps)
}

func TestParse_AllFlags(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"--output", "/out",
		"--backend-binary", "/backend",
		"--name", "My Backend",
		"--bundle-version", "1.2.3",
		"--platform", "linux-arm64",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, "My Backend", config.Name)
	assert.Equal(t, "1.2.3", config.Version)
	assert.Equal(t, "linux-arm64", config.Platform)
}

func TestParse_DefaultValues(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"--output", "/out",
		"--backend-binary", "/backend",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, "Convex Backend", config.Name)
	assert.Equal(t, "", config.Version)
	assert.Equal(t, "linux-x64", config.Platform)
}

func TestParse_MissingApp(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--output", "/out",
		"--backend-binary", "/backend",
	}

	_, err := Parse(args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one --app is required")
}

func TestParse_MissingOutput(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"--backend-binary", "/backend",
	}

	_, err := Parse(args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--output is required")
}

func TestParse_MissingBackendBinary(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"--output", "/out",
	}

	_, err := Parse(args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--backend-binary is required")
}

func TestParse_ShortOutputFlag(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"-o", "/short/output",
		"--backend-binary", "/backend",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, "/short/output", config.Output)
}

func TestParse_DockerImageFlag(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"--output", "/out",
		"--backend-binary", "/backend",
		"--docker-image", "ghcr.io/my-org/convex-predeploy:v1.0.0",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/my-org/convex-predeploy:v1.0.0", config.DockerImage)
}

func TestParse_DockerImageDefault(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"--output", "/out",
		"--backend-binary", "/backend",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	// Default is empty string, meaning predeploy.DefaultPredeployImage will be used
	assert.Equal(t, "", config.DockerImage)
}
