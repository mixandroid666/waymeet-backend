-- Per-post display aspect ratio (width / height), anchored to the first image
-- at creation time and clamped to Instagram's supported range [0.8, 1.91].
-- A single ratio per post drives a uniform carousel height so every slide in a
-- multi-image post shares the same shape. Legacy rows default to 1.0 (square).
ALTER TABLE posts ADD COLUMN aspect_ratio DOUBLE PRECISION NOT NULL DEFAULT 1.0;
