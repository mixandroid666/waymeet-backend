-- Auth: support phone signups, an account verification status, and OTP codes.

ALTER TABLE users ADD COLUMN phone TEXT UNIQUE;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;         -- phone-only signups
ALTER TABLE users ALTER COLUMN display_name DROP NOT NULL;  -- set during profile setup
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'pending_verification';
ALTER TABLE users ADD COLUMN verified_at TIMESTAMPTZ;
ALTER TABLE users ADD CONSTRAINT users_contact_present
    CHECK (email IS NOT NULL OR phone IS NOT NULL);

-- One-time passwords for registration (and later: password reset, etc.).
-- The code is NEVER stored in plaintext — only a bcrypt hash. Brute force is
-- bounded by a short expiry + an attempt cap enforced in the service.
CREATE TABLE otp_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT NOT NULL DEFAULT 'registration',
    code_hash   TEXT NOT NULL,
    attempts    INT NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_otp_user_purpose ON otp_codes (user_id, purpose, created_at DESC);
