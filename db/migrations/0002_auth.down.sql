DROP TABLE IF EXISTS otp_codes;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_contact_present;
ALTER TABLE users DROP COLUMN IF EXISTS verified_at;
ALTER TABLE users DROP COLUMN IF EXISTS status;
ALTER TABLE users DROP COLUMN IF EXISTS phone;
-- email / display_name NOT NULL are intentionally not restored (would fail if
-- any null rows now exist). Re-add manually if a full rollback is required.
