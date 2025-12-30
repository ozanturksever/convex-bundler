package main

import (
	"fmt"
	"os"

	"github.com/ozanturksever/convex-bundler/pkg/bundle"
	"github.com/ozanturksever/convex-bundler/pkg/cli"
	"github.com/ozanturksever/convex-bundler/pkg/credentials"
	"github.com/ozanturksever/convex-bundler/pkg/manifest"
	"github.com/ozanturksever/convex-bundler/pkg/predeploy"
	"github.com/ozanturksever/convex-bundler/pkg/selfhost"
	"github.com/ozanturksever/convex-bundler/pkg/version"
)

// Version information set by goreleaser ldflags
var (
	appVersion = "dev"
	commit     = "unknown"
	buildTime  = "unknown"
)

func main() {
	// Check for version flag early
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("convex-bundler %s\n", appVersion)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", buildTime)
		return
	}

	// Check if this is the selfhost subcommand
	if cli.IsSelfHostCommand(os.Args) {
		if err := runSelfHost(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(selfhost.ExitGeneralError)
		}
		return
	}

	if err := runBundle(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runBundle() error {
	// Parse CLI arguments
	config, err := cli.Parse(os.Args)
	if err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	fmt.Printf("Bundling Convex apps...\n")
	fmt.Printf("  Apps: %v\n", config.Apps)
	fmt.Printf("  Output: %s\n", config.Output)
	fmt.Printf("  Platform: %s\n", config.Platform)

	// Detect version
	detectedVersion, err := version.Detect(config.Apps[0], config.Version)
	if err != nil {
		return fmt.Errorf("failed to detect version: %w", err)
	}
	fmt.Printf("  Version: %s\n", detectedVersion)

	// Generate credentials
	fmt.Println("Generating credentials...")
	creds, err := credentials.Generate(config.Name)
	if err != nil {
		return fmt.Errorf("failed to generate credentials: %w", err)
	}

	// Create manifest
	mf := manifest.New(manifest.Options{
		Name:     config.Name,
		Version:  detectedVersion,
		Apps:     config.Apps,
		Platform: config.Platform,
	})

	// Run pre-deployment
	fmt.Println("Running pre-deployment...")
	predeployResult, err := predeploy.Run(predeploy.Options{
		Apps:          config.Apps,
		BackendBinary: config.BackendBinary,
		OutputDir:     config.Output,
		Platform:      config.Platform,
		DockerImage:   config.DockerImage,
	})
	if err != nil {
		return fmt.Errorf("pre-deployment failed: %w", err)
	}

	// Create bundle
	fmt.Println("Creating bundle...")
	err = bundle.Create(bundle.Options{
		OutputDir:     config.Output,
		BackendBinary: config.BackendBinary,
		DatabasePath:  predeployResult.DatabasePath,
		StoragePath:   predeployResult.StoragePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	if err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	fmt.Printf("\nBundle created successfully at: %s\n", config.Output)
	fmt.Println("Contents:")
	fmt.Println("  - backend (executable)")
	fmt.Println("  - convex.db (database)")
	fmt.Println("  - storage/ (file storage)")
	fmt.Println("  - manifest.json")
	fmt.Println("  - credentials.json")

	return nil
}

func runSelfHost() error {
	// Parse selfhost CLI arguments (skip "convex-bundler" and "selfhost" from args)
	config, err := cli.ParseSelfHost(os.Args[1:]) // Pass args starting from "selfhost"
	if err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	fmt.Println("Creating self-extracting executable...")
	fmt.Printf("  Bundle: %s\n", config.BundleDir)
	fmt.Printf("  Ops Binary: %s\n", config.OpsBinary)
	fmt.Printf("  Output: %s\n", config.Output)
	fmt.Printf("  Platform: %s\n", config.Platform)
	fmt.Printf("  Compression: %s\n", config.Compression)

	// Create self-extracting executable
	err = selfhost.Create(selfhost.CreateOptions{
		BundleDir:   config.BundleDir,
		OpsBinary:   config.OpsBinary,
		OutputPath:  config.Output,
		Platform:    config.Platform,
		Compression: config.Compression,
		OpsVersion:  config.OpsVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to create self-extracting executable: %w", err)
	}

	fmt.Printf("\nSelf-extracting executable created successfully at: %s\n", config.Output)
	fmt.Println("\nThe executable supports the following commands:")
	fmt.Println("  install    - Install from embedded bundle")
	fmt.Println("  extract    - Extract embedded bundle to a directory")
	fmt.Println("  info       - Display embedded bundle information")
	fmt.Println("  verify     - Verify embedded bundle integrity")

	return nil
}
