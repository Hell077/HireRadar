package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Client, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return &Client{pool: pool}, nil
}

func (c *Client) Ping(ctx context.Context) error { return c.pool.Ping(ctx) }
func (c *Client) Close()                         { c.pool.Close() }
func (c *Client) Pool() *pgxpool.Pool            { return c.pool }
