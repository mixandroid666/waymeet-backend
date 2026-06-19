-- Profile highlight photos: up to 6 ordered images shown on the profile slider.
CREATE TABLE profile_photos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    photo_url   TEXT NOT NULL,
    photo_order SMALLINT NOT NULL CHECK (photo_order BETWEEN 1 AND 6),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, photo_order)
);
CREATE INDEX idx_profile_photos_user ON profile_photos (user_id, photo_order);
