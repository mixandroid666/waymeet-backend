// Package chat owns 1:1 conversations and realtime messaging.
//
// Responsibilities:
//   - Conversations and message history (text, sticker, image, video, voice;
//     media carried as a URL produced by the media module).
//   - A WebSocket hub: each connected client registers with the hub; outgoing
//     messages publish to a Redis channel per conversation so delivery works
//     across multiple API instances (horizontal scale).
//   - Persisting messages to Postgres and triggering push via notification.
//
// Maps to the Flutter Chat list and chat-room screens.
package chat
