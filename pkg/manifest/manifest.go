package manifest

import (
	"encoding/json"
	"time"
)

// Manifest represents the bundle manifest
type Manifest struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Apps      []string `json:"apps"`
	Platform  string   `json:"platform"`
	CreatedAt string   `json:"createdAt"`
}

// Options for creating a new manifest
type Options struct {
	Name     string
	Version  string
	Apps     []string
	Platform string
}

// New creates a new Manifest with the given options
func New(opts Options) *Manifest {
	return &Manifest{
		Name:      opts.Name,
		Version:   opts.Version,
		Apps:      opts.Apps,
		Platform:  opts.Platform,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// ToJSON serializes the manifest to JSON
func (m *Manifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
