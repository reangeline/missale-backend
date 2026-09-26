package outbound

import "context"

// ImageUpload is what the browser needs to send one image straight to
// storage: a form POST to URL with Fields, then the file. PublicURL is where
// the app will read it.
type ImageUpload struct {
	URL       string            `json:"url"`
	Fields    map[string]string `json:"fields"`
	PublicURL string            `json:"publicUrl"`
	MaxBytes  int64             `json:"maxBytes"`
}

// ImageUploader authorizes one upload of an image to the content bucket,
// bounded in type and size, without the file passing through the API.
type ImageUploader interface {
	PrepareUpload(ctx context.Context, key, contentType string, maxBytes int64) (ImageUpload, error)
}
