ALTER TABLE messages
  ADD COLUMN msg_type  TEXT NOT NULL DEFAULT 'text',
  ADD COLUMN media_url TEXT;
