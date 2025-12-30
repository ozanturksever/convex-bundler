package main

import (
	"fmt"
	"os"

	"convex-bundler/pkg/bundle"
	"convex-bundler/pkg/cli"
	"convex-bundler/pkg/credentials"
	"convex-bundler/pkg/manifest"
	"convex-bundler/pkg/predeploy"
	"convex-bundler/pkg/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
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
