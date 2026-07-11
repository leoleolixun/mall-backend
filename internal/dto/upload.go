package dto

type UploadImageResponse struct {
	URL         string `json:"url"`
	Key         string `json:"key"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Provider    string `json:"provider"`
}
