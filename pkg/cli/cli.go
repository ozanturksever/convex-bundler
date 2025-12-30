package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Config holds the parsed CLI configuration
type Config struct {
	Apps          []string
	Output        string
	BackendBinary string
	Name          string
	Version       string
	Platform      string
	DockerImage   string
}

// ParseOptions configures the Parse function
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
