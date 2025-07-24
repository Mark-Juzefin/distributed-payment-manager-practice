package order_repo

import (
	"TestTaskJustPay/internal/controller/apperror"
	domain2 "TestTaskJustPay/internal/domain"
	"TestTaskJustPay/pkg"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	conn *pgxpool.Pool
}

func NewRepo(conn *pgxpool.Pool) *Repo {
	return &Repo{conn: conn}
}

func (r *Repo) FindById(ctx context.Context, id string) (domain2.Order, error) {
	row := r.conn.QueryRow(ctx, `SELECT id, user_id, status, created_at, updated_at FROM "order" WHERE id = $1`, id)

	order, err := parseOrderRow(row)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return domain2.Order{}, apperror.OrderNotFound
	}
	return order, err
}

func (r *Repo) FindByFilter(ctx context.Context, filter domain2.Filter) ([]domain2.Order, error) {
	rows, err := r.conn.Query(ctx, filterOrdersQuery(filter), filterOrdersArgs(filter))
	if err != nil {
		return nil, err
	}

	return parseOrderRows(rows)
}

func (r *Repo) GetEventsByOrderId(ctx context.Context, id string) ([]domain2.EventBase, error) {
	rows, err := r.conn.Query(ctx, getEventsQuery, id)
	if err != nil {
		return nil, err
	}

	return parseEventRows(rows)
}

// todo: create object UpdateOrderAndSaveEventTransaction
func (r *Repo) UpdateOrderAndSaveEvent(ctx context.Context, event domain2.Event) error {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if event.Status == domain2.StatusCreated {
		err = r.txCreateOrderByEvent(ctx, tx, event)
		if err != nil {
			fmt.Println("ERROR(txCreateOrderByEvent): ", err.Error())
			return err
		}

	} else {
		err = r.txUpdateOrder(ctx, tx, event)
		if err != nil {
			fmt.Println("ERROR(txUpdateOrder): ", err.Error())
			return err
		}
	}

	err = r.txCreateEvent(ctx, tx, event)
	if err != nil {
		fmt.Println("ERROR(txCreateEvent): ", err.Error())
		return err
	}

	return tx.Commit(ctx)

}

func (r *Repo) txUpdateOrder(ctx context.Context, tx pgx.Tx, event domain2.Event) error {
	currStatus, err := r.txGetStatus(ctx, tx, event)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperror.OrderNotFound
		}
		return err
	}

	if !currStatus.CanBeUpdatedTo(event.Status) {
		return apperror.UnappropriatedStatus
	}

	_, err = tx.Exec(ctx, `UPDATE "order" SET status = $1, updated_at = now() WHERE id = $2`, event.Status, event.OrderId)
	return err

}

func (r *Repo) txCreateEvent(ctx context.Context, tx pgx.Tx, event domain2.Event) error {
	query := `
	INSERT INTO "event" (id, order_id, user_id, status, created_at, updated_at, meta)
	VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := tx.Exec(ctx, query,
		event.EventId,
		event.OrderId,
		event.UserId,
		event.Status,
		event.CreatedAt,
		event.UpdatedAt,
		event.Meta,
	)
	if pkg.IsPgErrorUniqueViolation(err) {
		return apperror.EventAlreadyStored
	}
	return err
}
func (r *Repo) txCreateOrderByEvent(ctx context.Context, tx pgx.Tx, event domain2.Event) error {
	query := `
	INSERT INTO "order" (id, user_id, status, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5)`

	_, err := tx.Exec(ctx, query,
		event.OrderId,
		event.UserId,
		event.Status,
		event.CreatedAt,
		event.UpdatedAt,
	)
	return err
}

func (r *Repo) txGetStatus(ctx context.Context, tx pgx.Tx, event domain2.Event) (domain2.OrderStatus, error) {
	var rawStatus string
	err := tx.QueryRow(ctx, `SELECT status FROM "order" WHERE id = $1`, event.OrderId).Scan(&rawStatus)
	if err != nil {
		return "", err
	}

	return domain2.NewOrderStatus(rawStatus)
}
