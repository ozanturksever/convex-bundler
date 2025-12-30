// Package adminkey provides functions for generating admin keys for Convex
// self-hosted backend instances.
//
// This package generates admin keys compatible with the Convex backend's
// keybroker module. Keys are generated using AES-128-GCM-SIV encryption
// (RFC 8452) with keys derived using KBKDF-CTR-HMAC-SHA256 (NIST SP 800-108).
//
// # Basic Usage
//
//	// Generate a new random instance secret
//	secret, err := adminkey.GenerateSecret()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Or parse an existing secret (64-character hex string)
//	secret, err = adminkey.ParseSecret("4361726e69...")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Issue an admin key
//	key, err := adminkey.IssueAdminKey(secret, "my-instance", 0, false)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Key Types
//
// The package supports generating three types of keys:
//   - Standard admin keys: Full access to run queries, mutations, and actions
//   - Read-only admin keys: Can only run queries
//   - System keys: Used for internal Convex operations
//
// # Compatibility
//
// This implementation is fully compatible with:
//   - Convex self-hosted backend instances
//   - The official Rust gen-admin-key tool
//   - The Convex backend's keybroker module
package adminkey
