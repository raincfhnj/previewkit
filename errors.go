package previewkit

import "errors"

var (
	// ErrUnsupported means the file extension has no preview strategy.
	ErrUnsupported = errors.New("previewkit: unsupported file type")

	// ErrConverterRequired means an Office document was rendered without a
	// Converter configured on the Engine.
	ErrConverterRequired = errors.New("previewkit: office converter required")

	// ErrTooLarge means the file exceeds the configured inline-preview limit.
	ErrTooLarge = errors.New("previewkit: file too large for inline preview")

	// ErrBinary means a file classified as text contains binary data.
	ErrBinary = errors.New("previewkit: binary content")

	// ErrNotRegularFile means the path is a directory, device, or similar.
	ErrNotRegularFile = errors.New("previewkit: not a regular file")
)
