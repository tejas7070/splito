-- 004_invite_links.sql: Add shareable invite_code to trips and requested status to trip_members

ALTER TABLE trips ADD COLUMN IF NOT EXISTS invite_code VARCHAR(64) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_trips_invite_code ON trips (invite_code);

ALTER TABLE trip_members MODIFY COLUMN status ENUM('invited', 'active', 'rejected', 'left', 'removed', 'requested') NOT NULL DEFAULT 'invited';

UPDATE trips SET invite_code = CONCAT('splito_', SUBSTRING(MD5(CONCAT(id, NOW(), RAND())), 1, 12)) WHERE invite_code IS NULL OR invite_code = '';
