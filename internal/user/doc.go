// Package user owns user profiles and the follow graph.
//
// Responsibilities:
//   - Profile read/update (display name, avatar, bio, gender, birth date).
//   - Follow / unfollow and follower/following counts (counts cached in Redis).
//   - Profile timelines (a user's post history) by delegating to the feed module.
//
// Maps to the Flutter Profile and User-profile screens.
package user
