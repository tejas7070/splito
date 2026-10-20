-- Splito Authoritative Live Schema Dump for TiDB Cloud
CREATE DATABASE IF NOT EXISTS splito_db;
USE splito_db;

SET FOREIGN_KEY_CHECKS = 0;

-- Table structure for contributions
CREATE TABLE `contributions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `trip_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `amount` decimal(12,2) NOT NULL,
  `received_at` datetime NOT NULL,
  `payment_method` enum('cash','upi','bank_transfer','other') NOT NULL DEFAULT 'cash',
  `notes` varchar(500) DEFAULT NULL,
  `recorded_by` bigint unsigned NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `client_uuid` varchar(64) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_contributions_client_uuid` (`client_uuid`),
  KEY `fk_contributions_trip` (`trip_id`),
  KEY `fk_contributions_user` (`user_id`),
  KEY `fk_contributions_recorder` (`recorded_by`),
  CONSTRAINT `fk_contributions_recorder` FOREIGN KEY (`recorded_by`) REFERENCES `user_details` (`id`),
  CONSTRAINT `fk_contributions_trip` FOREIGN KEY (`trip_id`) REFERENCES `trips` (`id`),
  CONSTRAINT `fk_contributions_user` FOREIGN KEY (`user_id`) REFERENCES `user_details` (`id`),
  CONSTRAINT `chk_contribution_amount` CHECK ((`amount` > 0))
) ENGINE=InnoDB AUTO_INCREMENT=11 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for custom_parameters
CREATE TABLE `custom_parameters` (
  `param_key` varchar(100) NOT NULL,
  `param_value` varchar(255) NOT NULL,
  `description` varchar(255) DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`param_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for expense_claims
CREATE TABLE `expense_claims` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `trip_id` bigint unsigned NOT NULL,
  `claimant_id` bigint unsigned NOT NULL,
  `title` varchar(150) NOT NULL,
  `amount` decimal(12,2) NOT NULL,
  `category` varchar(50) NOT NULL DEFAULT 'other',
  `reason` text NOT NULL,
  `receipt_path` varchar(500) DEFAULT NULL,
  `status` enum('pending','approved','rejected','clarification_requested') NOT NULL DEFAULT 'pending',
  `reimbursement_status` enum('not_applicable','unpaid','paid') NOT NULL DEFAULT 'unpaid',
  `reviewed_by` bigint unsigned DEFAULT NULL,
  `reviewed_at` datetime DEFAULT NULL,
  `rejection_reason` text,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `client_uuid` varchar(64) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_claims_client_uuid` (`client_uuid`),
  KEY `fk_claims_reviewer` (`reviewed_by`),
  KEY `idx_trip_claim_status` (`trip_id`,`status`),
  KEY `idx_claimant` (`claimant_id`),
  CONSTRAINT `fk_claims_reviewer` FOREIGN KEY (`reviewed_by`) REFERENCES `user_details` (`id`),
  CONSTRAINT `fk_claims_trip` FOREIGN KEY (`trip_id`) REFERENCES `trips` (`id`),
  CONSTRAINT `fk_claims_user` FOREIGN KEY (`claimant_id`) REFERENCES `user_details` (`id`),
  CONSTRAINT `chk_claim_amount` CHECK ((`amount` > 0))
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for expenses
CREATE TABLE `expenses` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `trip_id` bigint unsigned NOT NULL,
  `title` varchar(150) NOT NULL,
  `category` enum('hotel','petrol','food','parking','travel','activities','shopping','other') NOT NULL DEFAULT 'other',
  `amount` decimal(12,2) NOT NULL,
  `paid_from` enum('group_funds','personal') NOT NULL,
  `paid_by` bigint unsigned NOT NULL,
  `recorded_by` bigint unsigned NOT NULL,
  `expense_date` datetime NOT NULL,
  `notes` text,
  `receipt_path` varchar(500) DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `client_uuid` varchar(64) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_expenses_client_uuid` (`client_uuid`),
  KEY `fk_expenses_paid_by` (`paid_by`),
  KEY `fk_expenses_recorded_by` (`recorded_by`),
  KEY `idx_trip_expense_date` (`trip_id`,`expense_date`),
  CONSTRAINT `fk_expenses_paid_by` FOREIGN KEY (`paid_by`) REFERENCES `user_details` (`id`),
  CONSTRAINT `fk_expenses_recorded_by` FOREIGN KEY (`recorded_by`) REFERENCES `user_details` (`id`),
  CONSTRAINT `fk_expenses_trip` FOREIGN KEY (`trip_id`) REFERENCES `trips` (`id`),
  CONSTRAINT `chk_expense_amount` CHECK ((`amount` > 0))
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for trip_invitations
CREATE TABLE `trip_invitations` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `trip_id` bigint unsigned NOT NULL,
  `inviter_user_id` bigint unsigned NOT NULL,
  `invitee_email` varchar(255) NOT NULL,
  `role` enum('munimji','co_reviewer','participant') NOT NULL DEFAULT 'participant',
  `token` varchar(64) NOT NULL,
  `status` enum('pending','accepted','rejected','expired','cancelled') NOT NULL DEFAULT 'pending',
  `expires_at` timestamp NULL DEFAULT NULL,
  `responded_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `token` (`token`),
  KEY `idx_trip_invitations_trip` (`trip_id`),
  KEY `idx_trip_invitations_email` (`invitee_email`),
  KEY `idx_trip_invitations_token` (`token`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for trip_members
CREATE TABLE `trip_members` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `trip_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `role` enum('munimji','co_reviewer','participant') NOT NULL DEFAULT 'participant',
  `status` enum('invited','active','rejected','left','removed') NOT NULL DEFAULT 'invited',
  `joined_at` timestamp NULL DEFAULT NULL,
  `removal_reason` text,
  `removed_by` bigint unsigned DEFAULT NULL,
  `removed_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_trip_member` (`trip_id`,`user_id`),
  KEY `fk_trip_members_user` (`user_id`),
  KEY `fk_trip_members_removed_by` (`removed_by`),
  CONSTRAINT `fk_trip_members_removed_by` FOREIGN KEY (`removed_by`) REFERENCES `user_details` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_trip_members_trip` FOREIGN KEY (`trip_id`) REFERENCES `trips` (`id`),
  CONSTRAINT `fk_trip_members_user` FOREIGN KEY (`user_id`) REFERENCES `user_details` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=24 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for trip_report_deliveries
CREATE TABLE `trip_report_deliveries` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `job_id` bigint NOT NULL,
  `recipient_user_id` bigint NOT NULL,
  `recipient_email` varchar(255) NOT NULL,
  `delivery_status` enum('pending','sending','sent','failed') NOT NULL DEFAULT 'pending',
  `attempts` int NOT NULL DEFAULT '0',
  `max_attempts` int NOT NULL DEFAULT '5',
  `next_retry_at` timestamp NULL DEFAULT NULL,
  `sent_at` timestamp NULL DEFAULT NULL,
  `last_error` text,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_deliveries_status_retry` (`delivery_status`,`next_retry_at`),
  KEY `idx_deliveries_job` (`job_id`),
  CONSTRAINT `fk_deliveries_job` FOREIGN KEY (`job_id`) REFERENCES `trip_report_jobs` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for trip_report_jobs
CREATE TABLE `trip_report_jobs` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `trip_id` bigint NOT NULL,
  `requested_by_user_id` bigint NOT NULL,
  `job_status` enum('queued','processing','completed','failed') NOT NULL DEFAULT 'queued',
  `report_type` enum('full_settlement','summary_ledger') NOT NULL DEFAULT 'full_settlement',
  `file_path` varchar(500) DEFAULT NULL,
  `file_name` varchar(255) DEFAULT NULL,
  `file_size_bytes` bigint DEFAULT NULL,
  `total_spend_paise` bigint NOT NULL DEFAULT '0',
  `total_contributions_paise` bigint NOT NULL DEFAULT '0',
  `error_message` text,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `completed_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_report_trip_status` (`trip_id`,`job_status`),
  KEY `idx_report_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for trips
CREATE TABLE `trips` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(150) NOT NULL,
  `start_location` varchar(255) NOT NULL,
  `destination` varchar(255) NOT NULL,
  `start_date` date NOT NULL,
  `end_date` date NOT NULL,
  `estimated_budget` decimal(12,2) NOT NULL DEFAULT '0.00',
  `created_by` bigint unsigned NOT NULL,
  `status` enum('planned','active','completed','cancelled','archived') NOT NULL DEFAULT 'planned',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `fk_trips_creator` (`created_by`),
  CONSTRAINT `fk_trips_creator` FOREIGN KEY (`created_by`) REFERENCES `user_details` (`id`),
  CONSTRAINT `chk_trip_dates` CHECK ((`end_date` >= `start_date`))
) ENGINE=InnoDB AUTO_INCREMENT=11 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Table structure for user_details
CREATE TABLE `user_details` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL,
  `email` varchar(255) NOT NULL,
  `password_hash` varchar(255) NOT NULL,
  `phone` varchar(20) DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `email` (`email`)
) ENGINE=InnoDB AUTO_INCREMENT=11 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

SET FOREIGN_KEY_CHECKS = 1;

