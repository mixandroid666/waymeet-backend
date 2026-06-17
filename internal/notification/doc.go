// Package notification delivers push notifications via FCM.
//
// Responsibilities:
//   - Register/unregister device FCM tokens per user.
//   - Fan out notifications (new message, new follower, post like) as async
//     worker tasks so the request path stays fast.
package notification
