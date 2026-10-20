-- TripLedger MySQL Initial Schema Migration (001_initial_schema.sql)

CREATE TABLE IF NOT EXISTS users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    full_name VARCHAR(100) NOT NULL,
    phone VARCHAR(30) NULL,
    is_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS otp_challenges (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NULL,
    email VARCHAR(255) NOT NULL,
    hashed_otp VARCHAR(255) NOT NULL,
    purpose VARCHAR(50) NOT NULL DEFAULT 'login',
    attempts INT DEFAULT 0,
    max_attempts INT DEFAULT 5,
    expires_at TIMESTAMP NOT NULL,
    consumed_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_otp_email (email),
    INDEX idx_otp_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS trips (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(150) NOT NULL,
    start_point VARCHAR(255) NOT NULL,
    destination VARCHAR(255) NOT NULL,
    start_at DATETIME NOT NULL,
    end_at DATETIME NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    budget_paise BIGINT NOT NULL DEFAULT 0,
    expected_contribution_paise BIGINT NOT NULL DEFAULT 0,
    status ENUM('draft', 'upcoming', 'active', 'completed', 'cancelled', 'archived') NOT NULL DEFAULT 'upcoming',
    creator_user_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_trips_creator FOREIGN KEY (creator_user_id) REFERENCES users(id),
    INDEX idx_trips_dates (start_at, end_at),
    INDEX idx_trips_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS trip_memberships (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role ENUM('munimji', 'co_reviewer', 'peer') NOT NULL DEFAULT 'peer',
    status ENUM('active', 'left', 'removed') NOT NULL DEFAULT 'active',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    left_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_trip_user (trip_id, user_id),
    CONSTRAINT fk_trip_memberships_trip FOREIGN KEY (trip_id) REFERENCES trips(id),
    CONSTRAINT fk_trip_memberships_user FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_user_membership_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS trip_invitations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT NOT NULL,
    inviter_user_id BIGINT NOT NULL,
    invitee_email VARCHAR(255) NOT NULL,
    role ENUM('co_reviewer', 'peer') NOT NULL DEFAULT 'peer',
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    status ENUM('pending', 'accepted', 'rejected', 'expired') NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP NOT NULL,
    responded_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_invitations_trip FOREIGN KEY (trip_id) REFERENCES trips(id),
    CONSTRAINT fk_invitations_inviter FOREIGN KEY (inviter_user_id) REFERENCES users(id),
    INDEX idx_invitation_email (invitee_email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS contributions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    amount_paise BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    recorded_by_user_id BIGINT NOT NULL,
    received_at DATETIME NOT NULL,
    status ENUM('recorded', 'reversed') NOT NULL DEFAULT 'recorded',
    reference_note VARCHAR(255) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_contributions_trip FOREIGN KEY (trip_id) REFERENCES trips(id),
    CONSTRAINT fk_contributions_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_contributions_recorder FOREIGN KEY (recorded_by_user_id) REFERENCES users(id),
    INDEX idx_contributions_trip (trip_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS expenses (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT NOT NULL,
    amount_paise BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    category VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    expense_date DATETIME NOT NULL,
    payer_user_id BIGINT NOT NULL,
    payment_source ENUM('group_fund', 'peer_out_of_pocket') NOT NULL DEFAULT 'group_fund',
    recorded_by_user_id BIGINT NOT NULL,
    status ENUM('recorded', 'reversed', 'adjusted') NOT NULL DEFAULT 'recorded',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_expenses_trip FOREIGN KEY (trip_id) REFERENCES trips(id),
    CONSTRAINT fk_expenses_payer FOREIGN KEY (payer_user_id) REFERENCES users(id),
    CONSTRAINT fk_expenses_recorder FOREIGN KEY (recorded_by_user_id) REFERENCES users(id),
    INDEX idx_expenses_trip_date (trip_id, expense_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS expense_claims (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT NOT NULL,
    claimant_user_id BIGINT NOT NULL,
    amount_paise BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    category VARCHAR(50) NOT NULL,
    reason TEXT NOT NULL,
    expense_date DATETIME NOT NULL,
    receipt_url VARCHAR(500) NULL,
    status ENUM('pending', 'needs_clarification', 'approved', 'rejected', 'withdrawn') NOT NULL DEFAULT 'pending',
    reimbursement_status ENUM('not_applicable', 'unpaid', 'partially_paid', 'paid') NOT NULL DEFAULT 'unpaid',
    reviewer_user_id BIGINT NULL,
    decision_reason TEXT NULL,
    decided_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_claims_trip FOREIGN KEY (trip_id) REFERENCES trips(id),
    CONSTRAINT fk_claims_claimant FOREIGN KEY (claimant_user_id) REFERENCES users(id),
    CONSTRAINT fk_claims_reviewer FOREIGN KEY (reviewer_user_id) REFERENCES users(id),
    INDEX idx_claims_trip_status (trip_id, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS reimbursements (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    claim_id BIGINT NOT NULL,
    trip_id BIGINT NOT NULL,
    recipient_user_id BIGINT NOT NULL,
    amount_paise BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    paid_by_user_id BIGINT NOT NULL,
    paid_at DATETIME NOT NULL,
    payment_reference VARCHAR(255) NULL,
    status ENUM('completed', 'reversed') NOT NULL DEFAULT 'completed',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_reimbursements_claim FOREIGN KEY (claim_id) REFERENCES expense_claims(id),
    CONSTRAINT fk_reimbursements_trip FOREIGN KEY (trip_id) REFERENCES trips(id),
    CONSTRAINT fk_reimbursements_recipient FOREIGN KEY (recipient_user_id) REFERENCES users(id),
    CONSTRAINT fk_reimbursements_payer FOREIGN KEY (paid_by_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ledger_entries (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT NOT NULL,
    entry_type ENUM('contribution', 'group_expense', 'peer_claim_approved', 'reimbursement_paid', 'refund_received', 'adjustment') NOT NULL,
    direction ENUM('inflow', 'outflow') NOT NULL,
    amount_paise BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'INR',
    source_entity_type VARCHAR(50) NOT NULL,
    source_entity_id BIGINT NOT NULL,
    actor_user_id BIGINT NOT NULL,
    notes VARCHAR(255) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_ledger_trip FOREIGN KEY (trip_id) REFERENCES trips(id),
    CONSTRAINT fk_ledger_actor FOREIGN KEY (actor_user_id) REFERENCES users(id),
    INDEX idx_ledger_trip (trip_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS audit_events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    trip_id BIGINT NULL,
    actor_user_id BIGINT NOT NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id BIGINT NOT NULL,
    details_json JSON NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_audit_trip (trip_id, created_at),
    INDEX idx_audit_entity (entity_type, entity_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    aggregate_id VARCHAR(100) NOT NULL,
    payload_json JSON NOT NULL,
    status ENUM('pending', 'processing', 'completed', 'failed') NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    last_error TEXT NULL,
    next_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_outbox_queue (status, next_attempt_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
