package database

import (
	"context"
	"time"
)

// ConnectTimeout bounds initial pings and is the default PostgreSQL connect timeout.
const ConnectTimeout = 10 * time.Second

// Ping checks the initial connection without imposing its timeout on migrations.
func Ping(ctx context.Context, ping func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, ConnectTimeout)
	defer cancel()
	return ping(ctx)
}
