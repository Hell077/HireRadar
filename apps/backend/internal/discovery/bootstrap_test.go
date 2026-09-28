package discovery

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type sourceSeedDB struct{ registered map[string]bool }

func (db *sourceSeedDB) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if !strings.Contains(query, "ON CONFLICT(id) DO NOTHING") || strings.Contains(strings.ToUpper(query), "UPDATE") {
		return pgconn.CommandTag{}, errUnexpectedSeedQuery
	}
	id := args[0].(string)
	if db.registered[id] {
		return pgconn.NewCommandTag("INSERT 0 0"), nil
	}
	db.registered[id] = true
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

var errUnexpectedSeedQuery = errors.New("unexpected source bootstrap query")

func TestRegisterDefaultSourcesIsIdempotent(t *testing.T) {
	db := &sourceSeedDB{registered: make(map[string]bool)}
	first, err := RegisterSources(context.Background(), db)
	if err != nil || first != len(DefaultSources) {
		t.Fatalf("first bootstrap inserted=%d err=%v", first, err)
	}
	second, err := RegisterSources(context.Background(), db)
	if err != nil || second != 0 {
		t.Fatalf("second bootstrap inserted=%d err=%v", second, err)
	}
}
