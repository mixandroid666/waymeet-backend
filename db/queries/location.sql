-- name: ListNearbyUsers :many
-- Find users within :radius_m metres of (:lng, :lat), filtered by gender and
-- age range, nearest first. Pass gender = '' to skip the gender filter.
-- Casts give sqlc concrete Go types instead of interface{}.
SELECT
    id,
    display_name,
    avatar_url,
    bio,
    gender,
    date_part('year', age(birth_date))::int AS age,
    ST_Distance(
        location,
        ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::float8, sqlc.arg(lat)::float8), 4326)::geography
    )::float8 AS distance_m
FROM users
WHERE location IS NOT NULL
  AND id <> sqlc.arg(viewer_id)
  AND ST_DWithin(
        location,
        ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::float8, sqlc.arg(lat)::float8), 4326)::geography,
        sqlc.arg(radius_m)::float8
      )
  AND (sqlc.arg(gender)::text = '' OR gender = sqlc.arg(gender)::text)
  AND (birth_date IS NULL OR date_part('year', age(birth_date)) BETWEEN sqlc.arg(min_age)::int AND sqlc.arg(max_age)::int)
ORDER BY distance_m ASC
LIMIT sqlc.arg(lim)::int;
