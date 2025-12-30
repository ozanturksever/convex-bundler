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
		Use:   "convex-bundler",
		Short: "Bundle Convex apps with a backend binary",
		Long:  "A CLI tool that bundles Convex apps and a pre-provided backend binary into a portable package.",
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
