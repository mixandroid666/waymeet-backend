# Ruammit — Login & Session API (Flutter integration guide)

This document specifies the **login / token / session** API and how to wire it
into the Flutter app. It builds on the registration flow
(`docs/auth-registration-api.md`) — a user must be **registered and verified**
before they can log in.

> Written for the Flutter-side developer. You don't need to read the backend
> code to integrate.

---

## 1. How auth works (token model)

Login returns **two tokens**:

| Token | Lifetime | Stored as | Used for |
|-------|----------|-----------|----------|
| **access token** (JWT) | 15 min | sent on every request | `Authorization: Bearer <access>` header |
| **refresh token** (opaque) | 30 days | kept secret on device | getting a new access token when it expires |

Flow:

```
LoginScreen ── POST /auth/login ──► { access_token, refresh_token, user }
   store both tokens securely
        │
        ├─ call protected APIs with:  Authorization: Bearer <access_token>
        │
        ├─ access token expired? (401 invalid_token / unauthorized)
        │      └─ POST /auth/refresh { refresh_token } ──► new pair, retry once
        │
        └─ logout ── POST /auth/logout { refresh_token }  (revokes it server-side)
```

The **access token is short-lived on purpose** — if it expires mid-session, call
`/auth/refresh` to get a new one without making the user log in again. Refresh
tokens **rotate**: each refresh returns a *new* refresh token and invalidates the
old one. Reusing an old refresh token revokes the whole session (theft defense),
so always replace the stored refresh token with the newest one.

---

## 2. Base URL

Same as registration: Android emulator `http://10.0.2.2:8080`, iOS sim / desktop
/ web `http://localhost:8080`. All endpoints under `/api/v1/auth`, JSON in/out.

---

## 3. Endpoints

### 3.1 `POST /api/v1/auth/login`

**Request**
```json
{
  "contact_type": "email",        // "email" | "phone"
  "contact": "you@example.com",
  "password": "secret123"
}
```
> The current Flutter login screen is email-only → always send `"email"`. The
> API also accepts `"phone"` (for users who registered by phone), so you can add
> a phone toggle later without backend changes.

**Response `200 OK`**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_at": "2026-06-17T18:11:10Z",          // access-token expiry
  "refresh_token": "I47xJBqiET4whkRCoFlhFzqb4Uc...",
  "refresh_expires_at": "2026-07-17T17:56:10Z",
  "user": { "id": "329316c1-dbb5-413c-89d3-4ec3de68ad64" }
}
```

### 3.2 `POST /api/v1/auth/refresh`

Exchange a refresh token for a fresh pair (rotates the refresh token).

**Request** `{ "refresh_token": "I47xJBqiET4..." }`
**Response `200 OK`** — identical shape to login. **Replace both stored tokens
with the returned ones.**

### 3.3 `POST /api/v1/auth/logout`

Revoke a refresh token server-side.

**Request** `{ "refresh_token": "I47xJBqiET4..." }`
**Response `204 No Content`** — then clear both tokens from device storage.

### 3.4 `GET /api/v1/auth/me` (protected — example)

Returns the caller's id; useful to validate a stored token on app start.

**Request header** `Authorization: Bearer <access_token>`
**Response `200 OK`** `{ "id": "329316c1-..." }`

---

## 4. Error responses

Same envelope as the rest of the API: `{ "error": "code", "message": "..." }`.

| HTTP | `error` | Endpoint | Meaning / suggested UI |
|------|---------|----------|------------------------|
| 400  | `validation_error` | login | Bad contact format or empty password — show under field |
| 401  | `invalid_credentials` | login | Wrong contact or password — "Incorrect email or password" |
| 403  | `account_not_verified` | login | Registered but not OTP-verified — route to the OTP screen |
| 401  | `invalid_token` | refresh | Refresh token expired/revoked/unknown — force re-login |
| 401  | `unauthorized` | protected routes | Missing/expired access token — try refresh, else re-login |
| 204  | — | logout | Success (idempotent; unknown token still returns 204) |

---

## 5. Flutter integration

### 5.1 Where this plugs in

- **`lib/screens/login_screen.dart`** — `_onLogin()` currently calls
  `pushReplacement(HomeScreen)` unconditionally. Change it to: validate the form,
  call `POST /auth/login`, store the tokens on success, then navigate to Home; on
  `invalid_credentials` show an error; on `account_not_verified` push the
  `OtpScreen` so the user can finish verifying.
- **Token storage** — use **`flutter_secure_storage`** (Keychain/Keystore), **not**
  `SharedPreferences`, for the refresh token. Add it with `flutter pub add
  flutter_secure_storage`.
- **Attaching the token** — centralize HTTP in an authenticated client that adds
  the `Authorization` header and, on a `401`, tries `/auth/refresh` once and
  replays the request. `dio` with an `InterceptorsWrapper` is the cleanest fit
  (`flutter pub add dio`); `http` works too with a manual wrapper.
- **App start** — on launch, if a refresh token exists, call `/auth/refresh` (or
  `/auth/me` with a stored access token) to decide whether to skip the login
  screen.

### 5.2 Minimal Dart example (extends the AuthApi from the registration doc)

```dart
class Tokens {
  Tokens(this.access, this.refresh);
  final String access;
  final String refresh;
}

extension AuthApiLogin on AuthApi {
  Future<Tokens> login({
    required bool isEmail,
    required String contact,
    required String password,
  }) async {
    final json = await _post('/api/v1/auth/login', {
      'contact_type': isEmail ? 'email' : 'phone',
      'contact': contact,
      'password': password,
    });
    return Tokens(json['access_token'] as String, json['refresh_token'] as String);
  }

  Future<Tokens> refresh(String refreshToken) async {
    final json = await _post('/api/v1/auth/refresh', {'refresh_token': refreshToken});
    return Tokens(json['access_token'] as String, json['refresh_token'] as String);
  }

  Future<void> logout(String refreshToken) =>
      _post('/api/v1/auth/logout', {'refresh_token': refreshToken});
}
```

Usage in `login_screen.dart`:
```dart
try {
  final tokens = await AuthApi().login(
    isEmail: true,
    contact: _emailController.text.trim(),
    password: _passwordController.text,
  );
  await secureStorage.write(key: 'refresh_token', value: tokens.refresh);
  // keep access token in memory (it's short-lived)
  if (!mounted) return;
  Navigator.of(context).pushReplacement(
    MaterialPageRoute(builder: (_) => const HomeScreen()),
  );
} on AuthException catch (e) {
  if (e.code == 'account_not_verified') {
    // push OtpScreen(contact: ..., isEmail: true)
  } else {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
  }
}
```

> `_post` and `AuthException` come from the registration doc's `AuthApi`. Reuse
> the same client.

### 5.3 The refresh-on-401 pattern (recommended)

```
request → if 401:
   refresh() using stored refresh_token
   if refresh OK:  save new tokens, retry the original request ONCE
   if refresh fails (invalid_token): wipe tokens, send user to LoginScreen
```

Refresh only **once** per request to avoid loops. With `dio`, do this in an
interceptor so every call gets it for free.

---

## 6. Behavior & limits (match these client-side)

| Rule | Value |
|------|-------|
| Access token TTL | 15 min (config `ACCESS_TOKEN_TTL`) |
| Refresh token TTL | 30 days (config `REFRESH_TOKEN_TTL`) |
| Refresh rotation | yes — old token invalid after use |
| Reuse of revoked refresh | revokes ALL the user's sessions |
| Token type | JWT (HS256) access · opaque random refresh (stored hashed) |
| Header format | `Authorization: Bearer <access_token>` |

---

## 7. Notes / future work

- **Social login** (Google/Facebook/TikTok buttons) is **not** wired yet — those
  buttons should stay stubbed until the OAuth endpoints land.
- The access-token secret is `JWT_SECRET` (env). It is a dev default now and must
  be a strong secret in production, or all tokens are forgeable.
- "Remember me" is effectively always on — the refresh token persists the session
  for 30 days. Add a toggle later if needed.
- `account_not_verified` is returned only after the password is correct, so it
  doesn't leak which accounts exist to an attacker guessing passwords.

## 8. Quick manual test (curl)

```bash
# (after registering + verifying you@example.com)
curl -s -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"contact_type":"email","contact":"you@example.com","password":"secret123"}'

# use the access_token:
curl -s localhost:8080/api/v1/auth/me -H "Authorization: Bearer <ACCESS_TOKEN>"
```
