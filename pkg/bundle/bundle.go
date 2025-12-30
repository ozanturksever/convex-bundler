package bundle

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ozanturksever/convex-bundler/pkg/credentials"
	"github.com/ozanturksever/convex-bundler/pkg/manifest"
)

// Options for creating a bundle
type Options struct {
	OutputDir     string
	BackendBinary string
	DatabasePath  string
	StoragePath   string
	Manifest      *manifest.Manifest
	Credentials   *credentials.Credentials
}

// Create assembles the final bundle directory
func Create(opts Options) error {
	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Copy backend binary
	backendDest := filepath.Join(opts.OutputDir, "backend")
	if err := copyFile(opts.BackendBinary, backendDest); err != nil {
		return fmt.Errorf("failed to copy backend binary: %w", err)
	}
	// Make it executable
	if err := os.Chmod(backendDest, 0755); err != nil {
		return fmt.Errorf("failed to make backend executable: %w", err)
	}

	// Copy database
	dbDest := filepath.Join(opts.OutputDir, "convex.db")
	if err := copyFile(opts.DatabasePath, dbDest); err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	// Copy/create storage directory
	storageDest := filepath.Join(opts.OutputDir, "storage")
	if err := copyDir(opts.StoragePath, storageDest); err != nil {
		return fmt.Errorf("failed to copy storage directory: %w", err)
	}

	// Write manifest.json
	manifestData, err := opts.Manifest.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}
	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest.json: %w", err)
	}

	// Write credentials.json
	credsData, err := opts.Credentials.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize credentials: %w", err)
	}
	credsPath := filepath.Join(opts.OutputDir, "credentials.json")
	if err := os.WriteFile(credsPath, credsData, 0644); err != nil {
		return fmt.Errorf("failed to write credentials.json: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Preserve permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// copyDir copies a directory from src to dst
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
