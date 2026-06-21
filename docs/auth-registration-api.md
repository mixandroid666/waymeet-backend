# waymeet â€” Registration API (Flutter integration guide)

This document specifies the **registration + OTP verification** API and how to
wire it into the Flutter app. It is written so another Claude (or any developer)
working on the **Flutter side** can integrate without reading the backend code.

> Scope: account creation only. **Login is not part of this API yet** â€” after
> verification the app returns to the login screen (matches the current
> `OtpScreen` behavior). A `POST /api/v1/auth/login` will come later.

---

## 1. Flow overview

The flow maps directly onto the existing Flutter screens:

```
RegisterScreen                         OtpScreen
â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€                          â”€â”€â”€â”€â”€â”€â”€â”€â”€
contact (email|phone) + password
        â”‚  POST /auth/register
        â–¼
   201 + OTP "sent"  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â–º  user enters 6-digit code
                                              â”‚  POST /auth/verify-otp
                                              â–¼
                                         200 verified  â”€â”€â–º back to LoginScreen
                                              â–²
                                       "Resend" â”‚ POST /auth/resend-otp
```

- **Register** creates a *pending* (unverified) account and sends a 6-digit code.
- **Verify** activates the account when the code is correct.
- **Resend** issues a fresh code (60-second cooldown).

Re-registering an **unverified** contact is allowed (it updates the password and
resends a code). Re-registering a **verified** contact returns `409`.

---

## 2. Base URL

| Environment | Base URL |
|-------------|----------|
| Local (emulator â†’ host) | Android emulator: `http://10.0.2.2:8080` Â· iOS sim/desktop/web: `http://localhost:8080` |
| Production | TBD |

All endpoints are under `/api/v1/auth`. All requests and responses are JSON
(`Content-Type: application/json`).

---

## 3. Endpoints

### 3.1 `POST /api/v1/auth/register`

Create a pending account and send an OTP.

**Request**
```json
{
  "contact_type": "email",          // "email" | "phone"
  "contact": "you@example.com",     // email address or phone number
  "password": "secret123"           // min 6 chars
}
```

**Response `201 Created`**
```json
{
  "message": "Verification code sent.",
  "contact": "you@example.com",     // normalized (lowercased email / digits-only phone)
  "expires_at": "2026-06-17T17:39:32.134014+07:00",
  "debug_code": "199317"            // DEV ONLY â€” omitted in production
}
```

> **`debug_code`** is returned only when the server is not in production, so you
> can test the full flow without a real email/SMS provider. Do not depend on it
> in production builds â€” it will be absent.

### 3.2 `POST /api/v1/auth/verify-otp`

Verify the code and activate the account.

**Request**
```json
{
  "contact_type": "email",
  "contact": "you@example.com",
  "code": "199317"                  // exactly 6 digits
}
```

**Response `200 OK`**
```json
{ "message": "Account verified. You can now log in." }
```

### 3.3 `POST /api/v1/auth/resend-otp`

Invalidate the previous code and send a new one (60s cooldown).

**Request**
```json
{ "contact_type": "email", "contact": "you@example.com" }
```

**Response `200 OK`** â€” same shape as register (`message`, `contact`,
`expires_at`, dev `debug_code`).

---

## 4. Error responses

All errors share one envelope:
```json
{ "error": "machine_code", "message": "Human-readable text" }
```

| HTTP | `error`                   | When | Suggested UI |
|------|---------------------------|------|--------------|
| 400  | `invalid_request`         | Malformed/unknown JSON | Generic error snackbar |
| 400  | `validation_error`        | Bad email/phone/password/code format | Show under the field |
| 400  | `invalid_code`            | Wrong OTP | "Incorrect code" under the OTP boxes |
| 400  | `code_expired`            | OTP older than 10 min | Prompt to Resend |
| 404  | `no_pending_registration` | Verify/resend for unknown or non-pending contact | Send back to Register |
| 409  | `already_verified`        | Contact already activated | "Account exists â€” please log in" |
| 429  | `too_many_attempts`       | > 5 wrong codes for one OTP | Prompt to Resend |
| 429  | `resend_cooldown`         | Resend within 60s | Disable Resend with a countdown |
| 500  | `internal_error`          | Server fault | Generic error snackbar |

**Rules of thumb for the UI:** branch on the `error` code (not the message);
the `message` is safe to display verbatim as a fallback.

---

## 5. Flutter integration

### 5.1 Where this plugs into existing screens

- **`lib/screens/register_screen.dart`** â€” `_onRegister()` currently navigates
  straight to `OtpScreen`. Change it to first call `POST /auth/register`, and
  only navigate on `201`. The screen already has `_contactType`, `_contactController`,
  and `_passwordController` â€” map them to the request fields.
- **`lib/screens/otp_screen.dart`** â€” `_verify()` currently just shows a snackbar.
  Change it to call `POST /auth/verify-otp` with the joined 6-digit `_code`, and
  `popUntil(isFirst)` only on `200`. `_onResend()` should call `POST /auth/resend-otp`.
  `OtpScreen` already receives `contact` and `isEmail` â€” derive `contact_type`
  from `isEmail`.

> Suggested: add an `ApiClient` (e.g. `lib/api/auth_api.dart`) using the
> `http` or `dio` package, plus a small `AuthException` carrying the `error`
> code so screens can branch. Keep the base URL in one config constant.

### 5.2 Request mapping

`_ContactType.email â†’ "email"`, `_ContactType.phone â†’ "phone"`. The backend
normalizes the contact, so it's fine to send the raw text the user typed; just
display `response.contact` if you want the normalized form.

### 5.3 Minimal Dart example (using `package:http`)

```dart
import 'dart:convert';
import 'package:http/http.dart' as http;

const String kApiBase = 'http://10.0.2.2:8080'; // Android emulator â†’ host

class AuthException implements Exception {
  AuthException(this.code, this.message);
  final String code;     // e.g. "invalid_code"
  final String message;  // human-readable
}

class AuthApi {
  final http.Client _client;
  AuthApi([http.Client? client]) : _client = client ?? http.Client();

  Future<Map<String, dynamic>> _post(String path, Map<String, dynamic> body) async {
    final res = await _client.post(
      Uri.parse('$kApiBase$path'),
      headers: const {'Content-Type': 'application/json'},
      body: jsonEncode(body),
    );
    final json = res.body.isEmpty
        ? <String, dynamic>{}
        : jsonDecode(res.body) as Map<String, dynamic>;
    if (res.statusCode >= 400) {
      throw AuthException(
        (json['error'] as String?) ?? 'unknown',
        (json['message'] as String?) ?? 'Something went wrong',
      );
    }
    return json;
  }

  Future<void> register({
    required bool isEmail,
    required String contact,
    required String password,
  }) =>
      _post('/api/v1/auth/register', {
        'contact_type': isEmail ? 'email' : 'phone',
        'contact': contact,
        'password': password,
      });

  Future<void> verify({
    required bool isEmail,
    required String contact,
    required String code,
  }) =>
      _post('/api/v1/auth/verify-otp', {
        'contact_type': isEmail ? 'email' : 'phone',
        'contact': contact,
        'code': code,
      });

  Future<void> resend({required bool isEmail, required String contact}) =>
      _post('/api/v1/auth/resend-otp', {
        'contact_type': isEmail ? 'email' : 'phone',
        'contact': contact,
      });
}
```

Usage in `register_screen.dart`:
```dart
try {
  await AuthApi().register(
    isEmail: _contactType == _ContactType.email,
    contact: _contactController.text.trim(),
    password: _passwordController.text,
  );
  if (!mounted) return;
  Navigator.of(context).push(MaterialPageRoute(
    builder: (_) => OtpScreen(
      contact: _contactController.text.trim(),
      isEmail: _contactType == _ContactType.email,
    ),
  ));
} on AuthException catch (e) {
  // e.code == 'already_verified' â†’ suggest logging in, etc.
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
}
```

> Add the `http` package to `waymeet-flutter/pubspec.yaml`
> (`flutter pub add http`) â€” it is not currently a dependency.

---

## 6. Behavior & limits (so the client can match)

| Rule | Value |
|------|-------|
| OTP length | 6 digits |
| OTP expiry | 10 minutes |
| Max wrong attempts per code | 5 (then `too_many_attempts`) |
| Resend cooldown | 60 seconds |
| Min password length | 6 (matches the Flutter validators) |
| Email normalization | trimmed + lowercased |
| Phone normalization | leading `+` kept, all non-digits stripped |

---

## 7. Notes / future work

- **No tokens issued yet.** Verification only flips the account to `active`.
  Login (and JWT access/refresh tokens) is the next endpoint.
- **No display name collected at signup** â€” the `users.display_name` column is
  nullable and set later during profile setup.
- OTP delivery is currently logged server-side only (dev). Real email/SMS
  providers plug into the `Sender` interface on the backend.
- Codes are stored only as bcrypt hashes; brute force is bounded by the expiry
  and attempt cap above.

## 8. Quick manual test (curl)

```bash
# register (grab debug_code from the response)
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"contact_type":"email","contact":"you@example.com","password":"secret123"}'

# verify (use the debug_code)
curl -s -X POST localhost:8080/api/v1/auth/verify-otp \
  -H 'Content-Type: application/json' \
  -d '{"contact_type":"email","contact":"you@example.com","code":"199317"}'
```
