package utils

import "mime"

var mimeTypes = map[string]string{
	".html": "text/html",
	".css":  "text/css",
	".js":   "application/javascript",
	".json": "application/json",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	// Add more as needed
}

func GetMimeType(ext string) string {
	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}
	return mime.TypeByExtension(ext)
}
