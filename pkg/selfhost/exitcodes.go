// Package selfhost provides functionality for creating and extracting self-extracting
// Convex backend executables. A self-extracting executable combines a convex-backend-ops
// binary with an embedded bundle (containing backend binary, database, storage, manifest,
// and credentials) into a single portable file.
//
// The package supports:
//   - Creating self-extracting executables from a bundle directory and ops binary
//   - Detecting if the current executable contains an embedded bundle
//   - Extracting embedded bundles to a directory
//   - Verifying bundle integrity via SHA256 checksum
//   - Reading embedded bundle metadata without extraction
package selfhost

// Exit codes for selfhost operations.
// These are used for consistent error reporting across the CLI.
const (
	// ExitSuccess indicates the operation completed successfully.
	ExitSuccess = 0

	// ExitGeneralError indicates a general/unspecified error occurred.
	ExitGeneralError = 1

	// ExitInvalidArguments indicates invalid command-line arguments were provided.
	ExitInvalidArguments = 2

	// ExitVerificationFailed indicates the bundle checksum verification failed.
	ExitVerificationFailed = 3

	// ExitPlatformMismatch indicates the bundle platform doesn't match the host.
	ExitPlatformMismatch = 4

	// ExitExtractionFailed indicates the bundle extraction failed.
	ExitExtractionFailed = 5

	// ExitInstallationFailed indicates the installation process failed.
	ExitInstallationFailed = 6
)
