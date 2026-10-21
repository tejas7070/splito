package services

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"tripledger/config"
	"tripledger/loaders"
	"tripledger/types"
	"tripledger/writers"
)

func TestClaimApproval_ConcurrentTwoGoroutines(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tl := loaders.NewTripLoader(db)
	el := loaders.NewExpenseLoader(db)
	ew := writers.NewExpenseWriter(db)
	cfg := &config.Config{}
	ls := NewLedgerService(db, cfg, tl, el, ew)

	// Expectations for Goroutine 1 (Winner):
	// Begin -> Query member (munimji, active) -> Query trip (active) -> Update claims (1 row affected) -> Commit
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT role, status FROM trip_members WHERE trip_id = \\? AND user_id = \\?").
		WithArgs(int64(1), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "status"}).AddRow("munimji", "active"))
	mock.ExpectQuery("SELECT status FROM trips WHERE id = \\?").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("active"))
	mock.ExpectExec("UPDATE expense_claims SET status = \\?, reviewed_by = \\?, rejection_reason = \\?, reviewed_at = NOW\\(\\) WHERE id = \\? AND trip_id = \\? AND status = 'pending' AND claimant_id <> \\?").
		WithArgs("approved", int64(1), "", int64(10), int64(1), int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// Claim load for notification
	mock.ExpectQuery("SELECT.*FROM expense_claims.*WHERE id = \\?").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "trip_id", "claimant_id", "title", "amount", "category", "reason", "status", "reimbursement_status", "created_at"}).
			AddRow(int64(10), int64(1), int64(3), "Petrol", 500.0, "Fuel", "Fuel", "approved", "unpaid", "2026-10-20 00:00:00"))

	// Expectations for Goroutine 2 (Conflict loser - already reviewed):
	// Begin -> Query member (co_reviewer, active) -> Query trip (active) -> Update claims (0 rows affected) -> Rollback
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT role, status FROM trip_members WHERE trip_id = \\? AND user_id = \\?").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "status"}).AddRow("co_reviewer", "active"))
	mock.ExpectQuery("SELECT status FROM trips WHERE id = \\?").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("active"))
	mock.ExpectExec("UPDATE expense_claims SET status = \\?, reviewed_by = \\?, rejection_reason = \\?, reviewed_at = NOW\\(\\) WHERE id = \\? AND trip_id = \\? AND status = 'pending' AND claimant_id <> \\?").
		WithArgs("approved", int64(2), "", int64(10), int64(1), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected!
	mock.ExpectRollback()

	ctx := context.Background()
	results := make([]error, 2)
	var wg sync.WaitGroup

	// Sequentially execute to match ordered mock or concurrent
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := &types.ClaimDecisionRequest{TripID: 1, ClaimID: 10}
		results[0] = ls.DecideClaim(ctx, 1, req, true)
	}()
	wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		req := &types.ClaimDecisionRequest{TripID: 1, ClaimID: 10}
		results[1] = ls.DecideClaim(ctx, 2, req, true)
	}()
	wg.Wait()

	if results[0] != nil {
		t.Errorf("expected first approval to succeed, got: %v", results[0])
	}
	if results[1] != ErrClaimConflict {
		t.Errorf("expected second approval to return ErrClaimConflict (409), got: %v", results[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestClaimApproval_SelfApprovalForbidden(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tl := loaders.NewTripLoader(db)
	el := loaders.NewExpenseLoader(db)
	ew := writers.NewExpenseWriter(db)
	cfg := &config.Config{}
	ls := NewLedgerService(db, cfg, tl, el, ew)

	// User 1 is Munimji and also the claimant
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT role, status FROM trip_members WHERE trip_id = \\? AND user_id = \\?").
		WithArgs(int64(1), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "status"}).AddRow("munimji", "active"))
	mock.ExpectQuery("SELECT status FROM trips WHERE id = \\?").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("active"))
	// Claimant_id <> reviewerID fails in SQL WHERE clause -> 0 rows affected
	mock.ExpectExec("UPDATE expense_claims SET status = \\?, reviewed_by = \\?, rejection_reason = \\?, reviewed_at = NOW\\(\\) WHERE id = \\? AND trip_id = \\? AND status = 'pending' AND claimant_id <> \\?").
		WithArgs("approved", int64(1), "", int64(20), int64(1), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	ctx := context.Background()
	req := &types.ClaimDecisionRequest{TripID: 1, ClaimID: 20}
	err = ls.DecideClaim(ctx, 1, req, true)

	if err != ErrClaimConflict {
		t.Errorf("expected ErrClaimConflict for self-approval, got: %v", err)
	}
}

func TestClaimApproval_NonMemberForbidden(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tl := loaders.NewTripLoader(db)
	el := loaders.NewExpenseLoader(db)
	ew := writers.NewExpenseWriter(db)
	cfg := &config.Config{}
	ls := NewLedgerService(db, cfg, tl, el, ew)

	// Case 1: User 99 is not in trip_members (sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT role, status FROM trip_members WHERE trip_id = \\? AND user_id = \\?").
		WithArgs(int64(1), int64(99)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	ctx := context.Background()
	req := &types.ClaimDecisionRequest{TripID: 1, ClaimID: 30}
	err = ls.DecideClaim(ctx, 99, req, true)
	if err == nil || err.Error() != "reviewer is not a member of this trip" {
		t.Errorf("expected non-member error, got: %v", err)
	}

	// Case 2: User 3 is a participant, not reviewer
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT role, status FROM trip_members WHERE trip_id = \\? AND user_id = \\?").
		WithArgs(int64(1), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "status"}).AddRow("participant", "active"))
	mock.ExpectRollback()

	err = ls.DecideClaim(ctx, 3, req, true)
	if err != ErrNotReviewer {
		t.Errorf("expected ErrNotReviewer (403), got: %v", err)
	}
}

func TestClaimApproval_CompletedTripConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tl := loaders.NewTripLoader(db)
	el := loaders.NewExpenseLoader(db)
	ew := writers.NewExpenseWriter(db)
	cfg := &config.Config{}
	ls := NewLedgerService(db, cfg, tl, el, ew)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT role, status FROM trip_members WHERE trip_id = \\? AND user_id = \\?").
		WithArgs(int64(1), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "status"}).AddRow("munimji", "active"))
	mock.ExpectQuery("SELECT status FROM trips WHERE id = \\?").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("completed"))
	mock.ExpectRollback()

	ctx := context.Background()
	req := &types.ClaimDecisionRequest{TripID: 1, ClaimID: 40}
	err = ls.DecideClaim(ctx, 1, req, true)
	if err != ErrTripCompletedNoOp {
		t.Errorf("expected ErrTripCompletedNoOp on completed trip, got: %v", err)
	}
}
