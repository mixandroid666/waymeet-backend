// Package feed owns posts, stories, likes and comments.
//
// Responsibilities:
//   - Create/read/delete posts (text + up to 6 images).
//   - Stories (ephemeral, 24h TTL).
//   - Likes and comments with counts.
//   - The home timeline. Start fan-out-on-read (join follows + posts); move to
//     fan-out-on-write (precomputed timeline rows via the worker) at scale.
//
// Maps to the Flutter Feed screen, PostCard and write-post composer.
package feed
