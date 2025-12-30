package credentials

import (
	"encoding/json"
	"fmt"

	adminkey "github.com/ozanturksever/convex-admin-key"
)

// Credentials holds the generated admin credentials
type Credentials struct {
	AdminKey       string `json:"adminKey"`
	InstanceSecret string `json:"instanceSecret"`
}

// Generate creates new secure admin credentials using the convex-admin-key library
func Generate(instanceName string) (*Credentials, error) {
	// Generate a new cryptographically secure instance secret
	secret, err := adminkey.GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate instance secret: %w", err)
	}

	// Issue an admin key for the instance
	// memberID=0 for generic admin key, isReadOnly=false for full access
	adminKey, err := adminkey.IssueAdminKey(secret, instanceName, 0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to issue admin key: %w", err)
	}

	return &Credentials{
		AdminKey:       adminKey,
		InstanceSecret: secret.String(),
	}, nil
}

// ToJSON serializes the credentials to JSON
func (c *Credentials) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
