// Package auth handles authentication and authorization.
//
// Responsibilities:
//   - Email/password signup & login (argon2/bcrypt hashing).
//   - Social login via OAuth2 (Google, Facebook, TikTok) using x/oauth2.
//   - Issuing and refreshing JWT access/refresh token pairs.
//   - The authentication middleware that resolves the current user from the
//     Authorization header for protected routes.
//
// Exposes: RegisterRoutes(mux, Service) and Middleware for other modules.
package auth
