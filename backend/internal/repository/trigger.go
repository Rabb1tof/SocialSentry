package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
)

// TriggerRepo is the storage contract for triggers.
type TriggerRepo interface {
	Create(ctx context.Context, p TriggerParams) (domain.Trigger, error)
	GetByID(ctx context.Context, id string) (domain.Trigger, error)
	ListByAccount(ctx context.Context, accountID string) ([]domain.Trigger, error)
	ListActiveByAccount(ctx context.Context, accountID string) ([]domain.Trigger, error)
	CountByAccount(ctx context.Context, accountID string) (int64, error)
	Update(ctx context.Context, id string, p TriggerParams) (domain.Trigger, error)
	Toggle(ctx context.Context, id string, active bool) error
	Delete(ctx context.Context, id string) error
}

// TriggerParams is the shared input shape for Create and Update.
type TriggerParams struct {
	AccountID           string
	Name                string
	IsActive            bool
	EventType           string
	MatchMode           string
	Keywords            []string
	KeywordsMode        string
	CaseSensitive       bool
	ReplyToComment      bool
	ReplyCommentText    *string
	SendPrivateReply    bool
	PrivateReplyText    *string
	SendDM              bool
	DMText              *string
	CheckSubscription   bool
	ReplyIfSubscribed   *string
	ReplyIfUnsubscribed *string
	CooldownSeconds     int32
	MaxRepliesPerUser   int32
	Priority            int32
	ReplyDelaySeconds   int32
}

type pgTriggerRepo struct {
	q *generated.Queries
}

// NewTriggerRepo returns a pgx-backed TriggerRepo.
func NewTriggerRepo(q *generated.Queries) TriggerRepo {
	return &pgTriggerRepo{q: q}
}

func (r *pgTriggerRepo) Create(ctx context.Context, p TriggerParams) (domain.Trigger, error) {
	accUID, err := uuidFromString(p.AccountID)
	if err != nil {
		return domain.Trigger{}, fmt.Errorf("repository.trigger.Create: %w", err)
	}
	if p.Keywords == nil {
		p.Keywords = []string{}
	}
	row, err := r.q.CreateTrigger(ctx, generated.CreateTriggerParams{
		AccountID:           accUID,
		Name:                p.Name,
		IsActive:            p.IsActive,
		EventType:           p.EventType,
		MatchMode:           p.MatchMode,
		Keywords:            p.Keywords,
		KeywordsMode:        p.KeywordsMode,
		CaseSensitive:       p.CaseSensitive,
		ReplyToComment:      p.ReplyToComment,
		ReplyCommentText:    p.ReplyCommentText,
		SendPrivateReply:    p.SendPrivateReply,
		PrivateReplyText:    p.PrivateReplyText,
		SendDm:              p.SendDM,
		DmText:              p.DMText,
		CheckSubscription:   p.CheckSubscription,
		ReplyIfSubscribed:   p.ReplyIfSubscribed,
		ReplyIfUnsubscribed: p.ReplyIfUnsubscribed,
		CooldownSeconds:     p.CooldownSeconds,
		MaxRepliesPerUser:   p.MaxRepliesPerUser,
		Priority:            p.Priority,
		ReplyDelaySeconds:   p.ReplyDelaySeconds,
	})
	if err != nil {
		return domain.Trigger{}, fmt.Errorf("repository.trigger.Create: %w", err)
	}
	return rowToTrigger(row), nil
}

func (r *pgTriggerRepo) GetByID(ctx context.Context, id string) (domain.Trigger, error) {
	uid, err := uuidFromString(id)
	if err != nil {
		return domain.Trigger{}, ErrNotFound
	}
	row, err := r.q.GetTriggerByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Trigger{}, ErrNotFound
		}
		return domain.Trigger{}, fmt.Errorf("repository.trigger.GetByID: %w", err)
	}
	return rowToTrigger(row), nil
}

func (r *pgTriggerRepo) ListByAccount(ctx context.Context, accountID string) ([]domain.Trigger, error) {
	uid, err := uuidFromString(accountID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := r.q.ListTriggersByAccount(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("repository.trigger.ListByAccount: %w", err)
	}
	out := make([]domain.Trigger, len(rows))
	for i, row := range rows {
		out[i] = rowToTrigger(row)
	}
	return out, nil
}

func (r *pgTriggerRepo) ListActiveByAccount(ctx context.Context, accountID string) ([]domain.Trigger, error) {
	uid, err := uuidFromString(accountID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := r.q.ListActiveTriggersByAccount(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("repository.trigger.ListActiveByAccount: %w", err)
	}
	out := make([]domain.Trigger, len(rows))
	for i, row := range rows {
		out[i] = rowToTrigger(row)
	}
	return out, nil
}

func (r *pgTriggerRepo) CountByAccount(ctx context.Context, accountID string) (int64, error) {
	uid, err := uuidFromString(accountID)
	if err != nil {
		return 0, ErrNotFound
	}
	return r.q.CountTriggersByAccount(ctx, uid)
}

func (r *pgTriggerRepo) Update(ctx context.Context, id string, p TriggerParams) (domain.Trigger, error) {
	uid, err := uuidFromString(id)
	if err != nil {
		return domain.Trigger{}, ErrNotFound
	}
	if p.Keywords == nil {
		p.Keywords = []string{}
	}
	row, err := r.q.UpdateTrigger(ctx, generated.UpdateTriggerParams{
		ID:                  uid,
		Name:                p.Name,
		IsActive:            p.IsActive,
		EventType:           p.EventType,
		MatchMode:           p.MatchMode,
		Keywords:            p.Keywords,
		KeywordsMode:        p.KeywordsMode,
		CaseSensitive:       p.CaseSensitive,
		ReplyToComment:      p.ReplyToComment,
		ReplyCommentText:    p.ReplyCommentText,
		SendPrivateReply:    p.SendPrivateReply,
		PrivateReplyText:    p.PrivateReplyText,
		SendDm:              p.SendDM,
		DmText:              p.DMText,
		CheckSubscription:   p.CheckSubscription,
		ReplyIfSubscribed:   p.ReplyIfSubscribed,
		ReplyIfUnsubscribed: p.ReplyIfUnsubscribed,
		CooldownSeconds:     p.CooldownSeconds,
		MaxRepliesPerUser:   p.MaxRepliesPerUser,
		Priority:            p.Priority,
		ReplyDelaySeconds:   p.ReplyDelaySeconds,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Trigger{}, ErrNotFound
		}
		return domain.Trigger{}, fmt.Errorf("repository.trigger.Update: %w", err)
	}
	return rowToTrigger(row), nil
}

func (r *pgTriggerRepo) Toggle(ctx context.Context, id string, active bool) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.ToggleTrigger(ctx, generated.ToggleTriggerParams{
		ID:       uid,
		IsActive: active,
	}); err != nil {
		return fmt.Errorf("repository.trigger.Toggle: %w", err)
	}
	return nil
}

func (r *pgTriggerRepo) Delete(ctx context.Context, id string) error {
	uid, err := uuidFromString(id)
	if err != nil {
		return ErrNotFound
	}
	if err := r.q.DeleteTrigger(ctx, uid); err != nil {
		return fmt.Errorf("repository.trigger.Delete: %w", err)
	}
	return nil
}

func rowToTrigger(row generated.Trigger) domain.Trigger {
	return domain.Trigger{
		ID:                  uuidToString(row.ID),
		AccountID:           uuidToString(row.AccountID),
		Name:                row.Name,
		IsActive:            row.IsActive,
		EventType:           row.EventType,
		MatchMode:           row.MatchMode,
		Keywords:            row.Keywords,
		KeywordsMode:        row.KeywordsMode,
		CaseSensitive:       row.CaseSensitive,
		ReplyToComment:      row.ReplyToComment,
		ReplyCommentText:    row.ReplyCommentText,
		SendPrivateReply:    row.SendPrivateReply,
		PrivateReplyText:    row.PrivateReplyText,
		SendDM:              row.SendDm,
		DMText:              row.DmText,
		CheckSubscription:   row.CheckSubscription,
		ReplyIfSubscribed:   row.ReplyIfSubscribed,
		ReplyIfUnsubscribed: row.ReplyIfUnsubscribed,
		CooldownSeconds:     int(row.CooldownSeconds),
		MaxRepliesPerUser:   int(row.MaxRepliesPerUser),
		Priority:            int(row.Priority),
		ReplyDelaySeconds:   int(row.ReplyDelaySeconds),
		CreatedAt:           timeFromTs(row.CreatedAt),
		UpdatedAt:           timeFromTs(row.UpdatedAt),
	}
}
