package pkg

import (
	"TestTaskJustPay/src/config"
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPgPool(cfg config.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DB.String)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
