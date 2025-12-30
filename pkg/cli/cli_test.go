package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_BasicFlags(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/tmp/app",
		"--output", "/tmp/out",
		"--backend-binary", "/tmp/backend",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"/tmp/app"}, config.Apps)
	assert.Equal(t, "/tmp/out", config.Output)
	assert.Equal(t, "/tmp/backend", config.BackendBinary)
	assert.Equal(t, "Convex Backend", config.Name) // default
	assert.Equal(t, "linux-x64", config.Platform)  // default
}

func TestParse_AllFlags(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/tmp/app1",
		"--app", "/tmp/app2",
		"--output", "/tmp/out",
		"--backend-binary", "/tmp/backend",
		"--name", "My Backend",
		"--bundle-version", "1.2.3",
		"--platform", "linux-arm64",
		"--docker-image", "custom:latest",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"/tmp/app1", "/tmp/app2"}, config.Apps)
	assert.Equal(t, "/tmp/out", config.Output)
	assert.Equal(t, "/tmp/backend", config.BackendBinary)
	assert.Equal(t, "My Backend", config.Name)
	assert.Equal(t, "1.2.3", config.Version)
	assert.Equal(t, "linux-arm64", config.Platform)
	assert.Equal(t, "custom:latest", config.DockerImage)
}

func TestParse_ShortFlags(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/tmp/app",
		"-o", "/tmp/out",
		"--backend-binary", "/tmp/backend",
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)

	assert.Equal(t, "/tmp/out", config.Output)
}

func TestParse_RequiredFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing --app",
			args:    []string{"convex-bundler", "--output", "/out", "--backend-binary", "/bin"},
			wantErr: "at least one --app is required",
		},
		{
			name:    "missing --output",
			args:    []string{"convex-bundler", "--app", "/app", "--backend-binary", "/bin"},
			wantErr: "--output is required",
		},
		{
			name:    "missing --backend-binary",
			args:    []string{"convex-bundler", "--app", "/app", "--output", "/out"},
			wantErr: "--backend-binary is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.args, ParseOptions{SkipValidation: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestParse_Validation(t *testing.T) {
	tmpDir := t.TempDir()

	appDir := filepath.Join(tmpDir, "app")
	require.NoError(t, os.MkdirAll(appDir, 0755))

	backendBinary := filepath.Join(tmpDir, "backend")
	require.NoError(t, os.WriteFile(backendBinary, []byte("mock"), 0755))

	t.Run("valid paths", func(t *testing.T) {
		args := []string{
			"convex-bundler",
			"--app", appDir,
			"--output", filepath.Join(tmpDir, "out"),
			"--backend-binary", backendBinary,
		}

		_, err := Parse(args)
		require.NoError(t, err)
	})

	t.Run("app does not exist", func(t *testing.T) {
		args := []string{
			"convex-bundler",
			"--app", filepath.Join(tmpDir, "nonexistent"),
			"--output", filepath.Join(tmpDir, "out"),
			"--backend-binary", backendBinary,
		}

		_, err := Parse(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "app directory does not exist")
	})

	t.Run("backend binary does not exist", func(t *testing.T) {
		args := []string{
			"convex-bundler",
			"--app", appDir,
			"--output", filepath.Join(tmpDir, "out"),
			"--backend-binary", filepath.Join(tmpDir, "nonexistent-backend"),
		}

		_, err := Parse(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "backend binary does not exist")
	})
}

func TestParse_InvalidPlatform(t *testing.T) {
	// Platform validation is currently lenient (no strict validation)
	// This test documents that behavior
	args := []string{
		"convex-bundler",
		"--app", "/tmp/app",
		"--output", "/tmp/out",
		"--backend-binary", "/tmp/backend",
		"--platform", "windows-x64", // Invalid but not validated
	}

	config, err := Parse(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, "windows-x64", config.Platform)
}

// TestParseSelfHost_AllFlags tests that all selfhost flags are parsed correctly
func TestParseSelfHost_AllFlags(t *testing.T) {
	args := []string{
		"selfhost",
		"--bundle", "/path/to/bundle",
		"--ops-binary", "/path/to/ops",
		"--output", "/path/to/output",
		"--platform", "linux-arm64",
		"--compression", "zstd",
		"--ops-version", "1.5.0",
	}

	config, err := ParseSelfHost(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)

	assert.Equal(t, "/path/to/bundle", config.BundleDir)
	assert.Equal(t, "/path/to/ops", config.OpsBinary)
	assert.Equal(t, "/path/to/output", config.Output)
	assert.Equal(t, "linux-arm64", config.Platform)
	assert.Equal(t, "zstd", config.Compression)
	assert.Equal(t, "1.5.0", config.OpsVersion)
}

// TestParseSelfHost_ShortFlags tests short flag variants
func TestParseSelfHost_ShortFlags(t *testing.T) {
	args := []string{
		"selfhost",
		"-b", "/path/to/bundle",
		"-o", "/path/to/ops",
		"--output", "/path/to/output",
		"-p", "linux-x64",
		"-c", "gzip",
	}

	config, err := ParseSelfHost(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)

	assert.Equal(t, "/path/to/bundle", config.BundleDir)
	assert.Equal(t, "/path/to/ops", config.OpsBinary)
	assert.Equal(t, "/path/to/output", config.Output)
	assert.Equal(t, "linux-x64", config.Platform)
	assert.Equal(t, "gzip", config.Compression)
}

// TestParseSelfHost_RequiredFlags tests validation of required flags
func TestParseSelfHost_RequiredFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing --bundle",
			args:    []string{"selfhost", "--ops-binary", "/ops", "--output", "/out", "--platform", "linux-x64"},
			wantErr: "--bundle is required",
		},
		{
			name:    "missing --ops-binary",
			args:    []string{"selfhost", "--bundle", "/bundle", "--output", "/out", "--platform", "linux-x64"},
			wantErr: "--ops-binary is required",
		},
		{
			name:    "missing --output",
			args:    []string{"selfhost", "--bundle", "/bundle", "--ops-binary", "/ops", "--platform", "linux-x64"},
			wantErr: "--output is required",
		},
		{
			name:    "missing --platform",
			args:    []string{"selfhost", "--bundle", "/bundle", "--ops-binary", "/ops", "--output", "/out"},
			wantErr: "--platform is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSelfHost(tt.args, ParseOptions{SkipValidation: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestParseSelfHost_InvalidPlatform tests validation of platform value
func TestParseSelfHost_InvalidPlatform(t *testing.T) {
	args := []string{
		"selfhost",
		"--bundle", "/bundle",
		"--ops-binary", "/ops",
		"--output", "/out",
		"--platform", "windows-x64",
	}

	_, err := ParseSelfHost(args, ParseOptions{SkipValidation: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid platform")
}

// TestParseSelfHost_InvalidCompression tests validation of compression value
func TestParseSelfHost_InvalidCompression(t *testing.T) {
	args := []string{
		"selfhost",
		"--bundle", "/bundle",
		"--ops-binary", "/ops",
		"--output", "/out",
		"--platform", "linux-x64",
		"--compression", "lz4",
	}

	_, err := ParseSelfHost(args, ParseOptions{SkipValidation: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid compression")
}

// TestParseSelfHost_Defaults tests default values
func TestParseSelfHost_Defaults(t *testing.T) {
	args := []string{
		"selfhost",
		"--bundle", "/bundle",
		"--ops-binary", "/ops",
		"--output", "/out",
		"--platform", "linux-x64",
	}

	config, err := ParseSelfHost(args, ParseOptions{SkipValidation: true})
	require.NoError(t, err)

	assert.Equal(t, "gzip", config.Compression, "default compression should be gzip")
	assert.Empty(t, config.OpsVersion, "ops version should be empty by default")
}

// TestParseSelfHost_Validation tests file existence validation
func TestParseSelfHost_Validation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid bundle dir and ops binary
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	opsBinary := filepath.Join(tmpDir, "ops")
	require.NoError(t, os.WriteFile(opsBinary, []byte("mock"), 0755))

	t.Run("valid paths", func(t *testing.T) {
		args := []string{
			"selfhost",
			"--bundle", bundleDir,
			"--ops-binary", opsBinary,
			"--output", filepath.Join(tmpDir, "output"),
			"--platform", "linux-x64",
		}

		_, err := ParseSelfHost(args)
		require.NoError(t, err)
	})

	t.Run("bundle dir does not exist", func(t *testing.T) {
		args := []string{
			"selfhost",
			"--bundle", filepath.Join(tmpDir, "nonexistent"),
			"--ops-binary", opsBinary,
			"--output", filepath.Join(tmpDir, "output"),
			"--platform", "linux-x64",
		}

		_, err := ParseSelfHost(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bundle directory does not exist")
	})

	t.Run("bundle path is not a directory", func(t *testing.T) {
		args := []string{
			"selfhost",
			"--bundle", opsBinary, // using the ops binary file as bundle path
			"--ops-binary", opsBinary,
			"--output", filepath.Join(tmpDir, "output"),
			"--platform", "linux-x64",
		}

		_, err := ParseSelfHost(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bundle path is not a directory")
	})

	t.Run("ops binary does not exist", func(t *testing.T) {
		args := []string{
			"selfhost",
			"--bundle", bundleDir,
			"--ops-binary", filepath.Join(tmpDir, "nonexistent-ops"),
			"--output", filepath.Join(tmpDir, "output"),
			"--platform", "linux-x64",
		}

		_, err := ParseSelfHost(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ops binary does not exist")
	})

	t.Run("ops binary is a directory", func(t *testing.T) {
		args := []string{
			"selfhost",
			"--bundle", bundleDir,
			"--ops-binary", bundleDir, // using the bundle dir as ops binary path
			"--output", filepath.Join(tmpDir, "output"),
			"--platform", "linux-x64",
		}

		_, err := ParseSelfHost(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ops binary path is a directory")
	})
}

// TestIsSelfHostCommand tests the selfhost command detection
func TestIsSelfHostCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "selfhost command",
			args: []string{"convex-bundler", "selfhost", "--bundle", "/bundle"},
			want: true,
		},
		{
			name: "main bundle command",
			args: []string{"convex-bundler", "--app", "/app", "--output", "/out"},
			want: false,
		},
		{
			name: "empty args",
			args: []string{},
			want: false,
		},
		{
			name: "only program name",
			args: []string{"convex-bundler"},
			want: false,
		},
		{
			name: "version flag",
			args: []string{"convex-bundler", "--version"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSelfHostCommand(tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}
