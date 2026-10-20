package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type OutboxEvent struct {
	ID          int64           `json:"id"`
	EventType   string          `json:"event_type"`
	AggregateID string          `json:"aggregate_id"`
	PayloadJSON json.RawMessage `json:"payload_json"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
}

type OutboxWorker struct {
	db     *sql.DB
	stopCh chan struct{}
}

func NewOutboxWorker(db *sql.DB) *OutboxWorker {
	return &OutboxWorker{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start launches the background loop picking up pending outbox events
func (w *OutboxWorker) Start() {
	ticker := time.NewTicker(3 * time.Second)
	log.Println("[OutboxWorker] Started background outbox processor")

	go func() {
		for {
			select {
			case <-ticker.C:
				w.processPendingEvents()
			case <-w.stopCh:
				ticker.Stop()
				log.Println("[OutboxWorker] Stopped outbox processor")
				return
			}
		}
	}()
}

// Stop gracefully shuts down the worker
func (w *OutboxWorker) Stop() {
	close(w.stopCh)
}

func (w *OutboxWorker) processPendingEvents() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Fetch pending events ready for processing
	query := `
		SELECT id, event_type, aggregate_id, payload_json, attempts, max_attempts
		FROM outbox_events
		WHERE status IN ('pending', 'failed') 
		  AND attempts < max_attempts 
		  AND next_attempt_at <= NOW()
		ORDER BY id ASC
		LIMIT 10
	`

	rows, err := w.db.QueryContext(ctx, query)
	if err != nil {
		return
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var ev OutboxEvent
		var payloadStr string
		if err := rows.Scan(&ev.ID, &ev.EventType, &ev.AggregateID, &payloadStr, &ev.Attempts, &ev.MaxAttempts); err != nil {
			continue
		}
		ev.PayloadJSON = json.RawMessage(payloadStr)
		events = append(events, ev)
	}

	// 2. Dispatch each event
	for _, ev := range events {
		w.dispatchEvent(ctx, ev)
	}
}

func (w *OutboxWorker) dispatchEvent(ctx context.Context, ev OutboxEvent) {
	// Mark processing
	_, _ = w.db.ExecContext(ctx, "UPDATE outbox_events SET status = 'processing', attempts = attempts + 1 WHERE id = ?", ev.ID)

	// Simulate event handler delivery (e.g. Email / Push notification dispatch)
	log.Printf("[OutboxWorker] Processing Event ID %d: Type='%s', AggregateID='%s'", ev.ID, ev.EventType, ev.AggregateID)

	// Mark completed
	_, err := w.db.ExecContext(ctx, "UPDATE outbox_events SET status = 'completed', updated_at = NOW() WHERE id = ?", ev.ID)
	if err != nil {
		log.Printf("[OutboxWorker] Failed to mark event %d completed: %v", ev.ID, err)
	}
}

// QueueEventTx inserts an event into the transactional outbox table inside the caller's transaction
func QueueEventTx(ctx context.Context, tx *sql.Tx, eventType, aggregateID string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	query := `
		INSERT INTO outbox_events (event_type, aggregate_id, payload_json, status, attempts, next_attempt_at)
		VALUES (?, ?, ?, 'pending', 0, NOW())
	`
	_, err = tx.ExecContext(ctx, query, eventType, aggregateID, string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}
	return nil
}
