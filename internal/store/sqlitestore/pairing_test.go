//go:build sqlite || sqliteonly

package sqlitestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestSQLitePairingStore_RequestPairingRefreshesMetadataOnReuse(t *testing.T) {
	ctx, pairingStore, db := newTestSQLitePairingStore(t)

	code1, err := pairingStore.RequestPairing(ctx, "group:C0AMLJ0K282", "slack-bot-nguWLCSEWRW", "C0AMLJ0K282", "default", nil)
	if err != nil {
		t.Fatalf("first RequestPairing error: %v", err)
	}

	code2, err := pairingStore.RequestPairing(ctx, "group:C0AMLJ0K282", "slack-bot-nguWLCSEWRW", "C0AMLJ0K282", "default", map[string]string{"chat_title": "cypress"})
	if err != nil {
		t.Fatalf("second RequestPairing error: %v", err)
	}
	if code1 != code2 {
		t.Fatalf("expected reused code, got %q then %q", code1, code2)
	}

	var metadataJSON string
	err = db.QueryRowContext(ctx, "SELECT COALESCE(metadata, '{}') FROM pairing_requests WHERE code = ?", code1).Scan(&metadataJSON)
	if err != nil {
		t.Fatalf("query metadata error: %v", err)
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		t.Fatalf("unmarshal metadata error: %v", err)
	}
	if got := metadata["chat_title"]; got != "cypress" {
		t.Fatalf("metadata chat_title = %q, want cypress", got)
	}
}

func newTestSQLitePairingStore(t *testing.T) (context.Context, *SQLitePairingStore, *sql.DB) {
	t.Helper()

	db, err := OpenDB(filepath.Join(t.TempDir(), "pairing.db"))
	if err != nil {
		t.Fatalf("OpenDB error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema error: %v", err)
	}

	return store.WithTenantID(context.Background(), store.MasterTenantID), NewSQLitePairingStore(db), db
}
