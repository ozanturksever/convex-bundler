package selfhost

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ozanturksever/convex-bundler/pkg/manifest"
)

// Magic markers for self-extracting executable format
var (
	// MagicStart is the marker that indicates the start of the embedded bundle section.
	// Must be exactly 20 bytes: "CONVEX_BUNDLE_START\x00"
	MagicStart = []byte("CONVEX_BUNDLE_START\x00")

	// MagicEnd is the marker that indicates the end of the embedded bundle section.
	// Must be exactly 18 bytes: "CONVEX_BUNDLE_END\x00"
	MagicEnd = []byte("CONVEX_BUNDLE_END\x00")
)

const (
	// MagicStartLen is the length of the start magic marker (20 bytes)
	MagicStartLen = 20

	// MagicEndLen is the length of the end magic marker (18 bytes)
	MagicEndLen = 18

	// HeaderLengthSize is the size of the header length prefix (4 bytes, big-endian)
	HeaderLengthSize = 4

	// FooterSize is the size of the footer containing the offset to MagicStart (8 bytes, little-endian uint64)
	FooterSize = 8

	// HeaderVersion is the current version of the header format
	HeaderVersion = "1.0.0"

	// HeaderFormat is the format identifier for self-host bundles
	HeaderFormat = "selfhost-v1"

	// CompressionGzip indicates gzip compression
	CompressionGzip = "gzip"

	// CompressionZstd indicates zstd compression
	CompressionZstd = "zstd"
)

// Header contains metadata about the self-extracting executable and its embedded bundle.
type Header struct {
	// Version is the header format version
	Version string `json:"version"`

	// Format is always "selfhost-v1"
	Format string `json:"format"`

	// Compression is the compression algorithm used ("gzip" or "zstd")
	Compression string `json:"compression"`

	// BundleSize is the uncompressed bundle size in bytes
	BundleSize int64 `json:"bundleSize"`

	// BundleChecksum is the SHA256 checksum of the compressed bundle (format: "sha256:hexstring")
	BundleChecksum string `json:"bundleChecksum"`

	// Manifest contains the embedded bundle manifest
	Manifest *manifest.Manifest `json:"manifest"`

	// OpsVersion is the version of the embedded convex-backend-ops binary
	OpsVersion string `json:"opsVersion"`

	// CreatedAt is the ISO 8601 timestamp of when the self-extracting executable was created
	CreatedAt string `json:"createdAt"`
}

// NewHeader creates a new Header with default values set.
func NewHeader() *Header {
	return &Header{
		Version:     HeaderVersion,
		Format:      HeaderFormat,
		Compression: CompressionGzip,
	}
}

// ToJSON serializes the header to JSON.
func (h *Header) ToJSON() ([]byte, error) {
	return json.MarshalIndent(h, "", "  ")
}

// FromJSON deserializes a header from JSON.
func (h *Header) FromJSON(data []byte) error {
	return json.Unmarshal(data, h)
}

// WriteHeader writes the header to the writer with a 4-byte big-endian length prefix.
// Returns the total number of bytes written (length prefix + JSON data).
func WriteHeader(w io.Writer, header *Header) (int, error) {
	// Serialize header to JSON
	data, err := header.ToJSON()
	if err != nil {
		return 0, fmt.Errorf("failed to serialize header: %w", err)
	}

	// Write length prefix (4 bytes, big-endian)
	lengthBuf := make([]byte, HeaderLengthSize)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))

	n1, err := w.Write(lengthBuf)
	if err != nil {
		return n1, fmt.Errorf("failed to write header length: %w", err)
	}

	// Write JSON data
	n2, err := w.Write(data)
	if err != nil {
		return n1 + n2, fmt.Errorf("failed to write header data: %w", err)
	}

	return n1 + n2, nil
}

// ReadHeader reads a length-prefixed header from the reader.
// It expects a 4-byte big-endian length prefix followed by JSON data.
func ReadHeader(r io.Reader) (*Header, error) {
	// Read length prefix
	lengthBuf := make([]byte, HeaderLengthSize)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		return nil, fmt.Errorf("failed to read header length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)

	// Sanity check on length (max 1MB for header)
	const maxHeaderSize = 1 << 20
	if length > maxHeaderSize {
		return nil, fmt.Errorf("header size %d exceeds maximum allowed size %d", length, maxHeaderSize)
	}

	// Read header data
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("failed to read header data: %w", err)
	}

	// Parse JSON
	header := &Header{}
	if err := header.FromJSON(data); err != nil {
		return nil, fmt.Errorf("failed to parse header JSON: %w", err)
	}

	return header, nil
}

// Validate checks that the header has all required fields and valid values.
func (h *Header) Validate() error {
	if h.Version == "" {
		return fmt.Errorf("header version is required")
	}
	if h.Format != HeaderFormat {
		return fmt.Errorf("invalid header format: expected %q, got %q", HeaderFormat, h.Format)
	}
	if h.Compression != CompressionGzip && h.Compression != CompressionZstd {
		return fmt.Errorf("invalid compression: expected %q or %q, got %q", CompressionGzip, CompressionZstd, h.Compression)
	}
	if h.BundleSize <= 0 {
		return fmt.Errorf("bundle size must be positive")
	}
	if h.BundleChecksum == "" {
		return fmt.Errorf("bundle checksum is required")
	}
	if h.Manifest == nil {
		return fmt.Errorf("manifest is required")
	}
	if h.CreatedAt == "" {
		return fmt.Errorf("createdAt is required")
	}
	return nil
}
