package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Config holds the parsed CLI configuration for the main bundle command
type Config struct {
	Apps          []string
	Output        string
	BackendBinary string
	Name          string
	Version       string
	Platform      string
	DockerImage   string
}

// SelfHostConfig holds the parsed CLI configuration for the selfhost subcommand
type SelfHostConfig struct {
	// BundleDir is the path to the convex-bundler output directory
	BundleDir string

	// OpsBinary is the path to the convex-backend-ops binary
	OpsBinary string

	// Output is the output path for the self-extracting executable
	Output string

	// Platform is the target platform (e.g., "linux-x64", "linux-arm64")
	Platform string

	// Compression is the compression algorithm ("gzip" or "zstd")
	Compression string

	// OpsVersion is an optional version string for the ops binary (for metadata)
	OpsVersion string
}

// ParseOptions configures the Parse and ParseSelfHost functions
type ParseOptions struct {
	SkipValidation bool // Skip file existence validation (for testing)
}

// Parse parses command-line arguments and returns a Config
func Parse(args []string, opts ...ParseOptions) (*Config, error) {
	var parseOpts ParseOptions
	if len(opts) > 0 {
		parseOpts = opts[0]
	}
	config := &Config{}

	cmd := &cobra.Command{
		Use:   "convex-bundler [flags]",
		Short: "Bundle Convex apps with a backend binary",
		Long: `convex-bundler bundles Convex apps and a pre-provided backend binary into a 
portable, self-contained package ready for deployment.

The bundler performs the following steps:
  1. Validates the Convex app directory and backend binary
  2. Detects version from git tags, package.json, or CLI override
  3. Runs pre-deployment to initialize the database with your schema
  4. Generates secure credentials (admin key and instance secret)
  5. Creates the final bundle with all necessary files

The output bundle contains:
  - backend         The Convex backend executable
  - convex.db       Pre-initialized SQLite database
  - storage/        File storage directory
  - manifest.json   Bundle metadata
  - credentials.json  Admin key and instance secret`,
		Example: `  # Basic usage with required flags
  convex-bundler --app ./my-app --output ./bundle --backend-binary ./convex-local-backend

  # Bundle multiple apps
  convex-bundler --app ./app1 --app ./app2 -o ./bundle --backend-binary ./backend

  # Specify custom name and version
  convex-bundler --app ./my-app -o ./bundle --backend-binary ./backend \
    --name "My Convex App" --bundle-version 1.0.0

  # Target ARM64 Linux platform
  convex-bundler --app ./my-app -o ./bundle --backend-binary ./backend-arm64 \
    --platform linux-arm64

  # Use custom Docker image for pre-deployment
  convex-bundler --app ./my-app -o ./bundle --backend-binary ./backend \
    --docker-image ghcr.io/my-org/convex-predeploy:v1.0.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().StringSliceVar(&config.Apps, "app", []string{}, "Path to Convex app directory (can be specified multiple times)")
	cmd.Flags().StringVarP(&config.Output, "output", "o", "", "Output path for the bundle directory")
	cmd.Flags().StringVar(&config.BackendBinary, "backend-binary", "", "Path to the convex-local-backend binary")
	cmd.Flags().StringVar(&config.Name, "name", "Convex Backend", "Display name")
	cmd.Flags().StringVar(&config.Version, "bundle-version", "", "Bundle version override (semver)")
	cmd.Flags().StringVar(&config.Platform, "platform", "linux-x64", "Target platform: linux-x64, linux-arm64")
	cmd.Flags().StringVar(&config.DockerImage, "docker-image", "", "Docker image for pre-deployment (default: convex-predeploy:latest)")

	cmd.SetArgs(args[1:]) // Skip program name
	if err := cmd.Execute(); err != nil {
		return nil, err
	}

	// Validate required flags
	if len(config.Apps) == 0 {
		return nil, errors.New("at least one --app is required")
	}
	if config.Output == "" {
		return nil, errors.New("--output is required")
	}
	if config.BackendBinary == "" {
		return nil, errors.New("--backend-binary is required")
	}

	// Validate that apps and backend binary exist (unless skipped)
	if !parseOpts.SkipValidation {
		for _, app := range config.Apps {
			if _, err := os.Stat(app); os.IsNotExist(err) {
				return nil, fmt.Errorf("app directory does not exist: %s", app)
			}
		}
		if _, err := os.Stat(config.BackendBinary); os.IsNotExist(err) {
			return nil, fmt.Errorf("backend binary does not exist: %s", config.BackendBinary)
		}
	}

	return config, nil
}

// ParseSelfHost parses command-line arguments for the selfhost subcommand
func ParseSelfHost(args []string, opts ...ParseOptions) (*SelfHostConfig, error) {
	var parseOpts ParseOptions
	if len(opts) > 0 {
		parseOpts = opts[0]
	}
	config := &SelfHostConfig{}

	cmd := &cobra.Command{
		Use:   "convex-bundler selfhost [flags]",
		Short: "Create a self-extracting executable from a bundle",
		Long: `Create a self-extracting executable that combines a convex-backend-ops binary
with an embedded bundle. The resulting executable can install, extract, verify,
and display info about the embedded bundle.

The self-extracting executable contains:
  - convex-backend-ops binary (base executable)
  - Compressed bundle (tar.gz) with:
    - backend binary
    - convex.db (pre-initialized database)
    - storage/ directory
    - manifest.json
    - credentials.json`,
		Example: `  # Create self-extracting executable
  convex-bundler selfhost --bundle ./bundle --ops-binary ./convex-backend-ops \
    --output ./my-backend-selfhost --platform linux-x64

  # With zstd compression
  convex-bundler selfhost -b ./bundle -o ./convex-backend-ops \
    --output ./my-backend-selfhost -p linux-x64 -c zstd`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().StringVarP(&config.BundleDir, "bundle", "b", "", "Path to convex-bundler output directory")
	cmd.Flags().StringVarP(&config.OpsBinary, "ops-binary", "o", "", "Path to convex-backend-ops binary")
	cmd.Flags().StringVar(&config.Output, "output", "", "Output path for self-extracting executable")
	cmd.Flags().StringVarP(&config.Platform, "platform", "p", "", "Target platform: linux-x64, linux-arm64")
	cmd.Flags().StringVarP(&config.Compression, "compression", "c", "gzip", "Compression algorithm: gzip, zstd")
	cmd.Flags().StringVar(&config.OpsVersion, "ops-version", "", "Version of the ops binary (for metadata)")

	cmd.SetArgs(args[1:]) // Skip program name (or "selfhost" subcommand)
	if err := cmd.Execute(); err != nil {
		return nil, err
	}

	// Validate required flags
	if config.BundleDir == "" {
		return nil, errors.New("--bundle is required")
	}
	if config.OpsBinary == "" {
		return nil, errors.New("--ops-binary is required")
	}
	if config.Output == "" {
		return nil, errors.New("--output is required")
	}
	if config.Platform == "" {
		return nil, errors.New("--platform is required")
	}

	// Validate platform value
	validPlatforms := map[string]bool{
		"linux-x64":   true,
		"linux-arm64": true,
	}
	if !validPlatforms[config.Platform] {
		return nil, fmt.Errorf("invalid platform %q: must be linux-x64 or linux-arm64", config.Platform)
	}

	// Validate compression value
	validCompressions := map[string]bool{
		"gzip": true,
		"zstd": true,
	}
	if !validCompressions[config.Compression] {
		return nil, fmt.Errorf("invalid compression %q: must be gzip or zstd", config.Compression)
	}

	// Validate that bundle directory and ops binary exist (unless skipped)
	if !parseOpts.SkipValidation {
		info, err := os.Stat(config.BundleDir)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("bundle directory does not exist: %s", config.BundleDir)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to access bundle directory: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("bundle path is not a directory: %s", config.BundleDir)
		}

		info, err = os.Stat(config.OpsBinary)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("ops binary does not exist: %s", config.OpsBinary)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to access ops binary: %w", err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("ops binary path is a directory: %s", config.OpsBinary)
		}
	}

	return config, nil
}

// IsSelfHostCommand checks if the args indicate the selfhost subcommand
func IsSelfHostCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	return args[1] == "selfhost"
}
