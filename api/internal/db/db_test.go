package db

import (
	"context"
	"testing"

	"github.com/ante-rg/ante-app/api/internal/testdb"
)

func TestMigrate_FreshDatabase(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, testdb.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, pool); err != nil {
			t.Fatalf("migrate call %d: %v", i+1, err)
		}
	}
}
