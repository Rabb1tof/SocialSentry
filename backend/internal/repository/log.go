package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// LogRepo is the storage contract for trigger event logs.
type LogRepo interface {
	Create(ctx context.Context, p CreateLogParams) (domain.TriggerLog, error)
	ListByAccount(ctx context.Context, accountID string, limit, offset int) ([]domain.TriggerLog, int64, error)
	ListByTrigger(ctx context.Context, triggerID string, limit, offset int) ([]domain.TriggerLog, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]domain.TriggerLog, error)
}

// CreateLogParams is the input to LogRepo.Create.
type CreateLogParams struct {
	TriggerID       string
	AccountID       string
	EventType       string
	PlatformEventID *string
	SenderID        string
	SenderUsername  *string
	IncomingText    *string
	MatchedKeyword  *string
	ActionTaken     string
	ErrorMessage    *string
}

type pgLogRepo struct {
	q *generated.Queries
}

// NewLogRepo returns a pgx-backed LogRepo.
func NewLogRepo(q *generated.Queries) LogRepo {
	return &pgLogRepo{q: q}
}

func (r *pgLogRepo) Create(ctx context.Context, p CreateLogParams) (domain.TriggerLog, error) {
	// An empty TriggerID is a deliberate NULL: skipped/ingress events (cooldown,
	// max_replies_reached, no_action_text) are logged without a firing trigger.
	var tid pgtype.UUID
	if p.TriggerID != "" {
		var err error
		tid, err = uuidFromString(p.TriggerID)
		if err != nil {
			return domain.TriggerLog{}, fmt.Errorf("repository.log.Create trigger_id: %w", err)
		}
	}
	aid, err := uuidFromString(p.AccountID)
	if err != nil {
		return domain.TriggerLog{}, fmt.Errorf("repository.log.Create account_id: %w", err)
	}
	row, err := r.q.CreateTriggerLog(ctx, generated.CreateTriggerLogParams{
		TriggerID:       tid,
		AccountID:       aid,
		EventType:       p.EventType,
		PlatformEventID: p.PlatformEventID,
		SenderID:        p.SenderID,
		SenderUsername:  p.SenderUsername,
		IncomingText:    p.IncomingText,
		MatchedKeyword:  p.MatchedKeyword,
		ActionTaken:     p.ActionTaken,
		ErrorMessage:    p.ErrorMessage,
	})
	if err != nil {
		return domain.TriggerLog{}, fmt.Errorf("repository.log.Create: %w", err)
	}
	return rowToLog(row), nil
}

func (r *pgLogRepo) ListByAccount(ctx context.Context, accountID string, limit, offset int) ([]domain.TriggerLog, int64, error) {
	uid, err := uuidFromString(accountID)
	if err != nil {
		return nil, 0, ErrNotFound
	}
	rows, err := r.q.ListLogsByAccount(ctx, generated.ListLogsByAccountParams{
		AccountID: uid,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("repository.log.ListByAccount: %w", err)
	}
	total, err := r.q.CountLogsByAccount(ctx, uid)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.log.ListByAccount count: %w", err)
	}
	out := make([]domain.TriggerLog, len(rows))
	for i, row := range rows {
		out[i] = rowToLog(row)
	}
	return out, total, nil
}

// ListByUser returns recent logs across every account owned by the user (newest first).
func (r *pgLogRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]domain.TriggerLog, error) {
	uid, err := uuidFromString(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := r.q.ListLogsByUser(ctx, generated.ListLogsByUserParams{
		UserID: uid,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("repository.log.ListByUser: %w", err)
	}
	out := make([]domain.TriggerLog, len(rows))
	for i, row := range rows {
		out[i] = rowToLog(row.TriggerLog)
	}
	return out, nil
}

func (r *pgLogRepo) ListByTrigger(ctx context.Context, triggerID string, limit, offset int) ([]domain.TriggerLog, error) {
	uid, err := uuidFromString(triggerID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := r.q.ListLogsByTrigger(ctx, generated.ListLogsByTriggerParams{
		TriggerID: uid,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("repository.log.ListByTrigger: %w", err)
	}
	out := make([]domain.TriggerLog, len(rows))
	for i, row := range rows {
		out[i] = rowToLog(row)
	}
	return out, nil
}

func rowToLog(row generated.TriggerLog) domain.TriggerLog {
	return domain.TriggerLog{
		ID:              uuidToString(row.ID),
		TriggerID:       uuidToString(row.TriggerID),
		AccountID:       uuidToString(row.AccountID),
		EventType:       row.EventType,
		PlatformEventID: row.PlatformEventID,
		SenderID:        row.SenderID,
		SenderUsername:  row.SenderUsername,
		IncomingText:    row.IncomingText,
		MatchedKeyword:  row.MatchedKeyword,
		ActionTaken:     row.ActionTaken,
		ErrorMessage:    row.ErrorMessage,
		CreatedAt:       timeFromTs(row.CreatedAt),
	}
}
