ALTER TABLE trips
    ADD COLUMN start_latitude DECIMAL(10,7) NULL,
    ADD COLUMN start_longitude DECIMAL(10,7) NULL,
    ADD COLUMN destination_latitude DECIMAL(10,7) NULL,
    ADD COLUMN destination_longitude DECIMAL(10,7) NULL;

CREATE TABLE trip_stops (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT UNSIGNED NOT NULL,
    position INT NOT NULL,
    name VARCHAR(255) NOT NULL,
    latitude DECIMAL(10,7) NOT NULL,
    longitude DECIMAL(10,7) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_trip_stop_position (trip_id, position),
    CONSTRAINT fk_trip_stops_trip FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
