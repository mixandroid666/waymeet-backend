// Package media handles user-uploaded images, video and voice.
//
// Responsibilities:
//   - Issue presigned S3/MinIO upload URLs so client bytes never pass through
//     the API.
//   - Enqueue async processing on the worker: image thumbnails, video
//     transcoding (ffmpeg), voice format normalization.
//   - Resolve stored objects to CDN-backed delivery URLs.
package media
