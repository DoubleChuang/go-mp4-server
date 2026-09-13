package videoserver

import (
	"path/filepath"
	"testing"
)

func newTestTotpStore(t *testing.T) *TotpStore {
	t.Helper()
	return NewTotpStore(filepath.Join(t.TempDir(), "2fa.json"))
}

func TestTotpStoreEnabledMissingFile(t *testing.T) {
	store := newTestTotpStore(t)
	enabled, err := store.Enabled("admin")
	if err != nil {
		t.Fatalf("Enabled() error: %v", err)
	}
	if enabled {
		t.Fatal("expected disabled when no 2fa.json exists")
	}
}

func TestTotpStoreSaveSecretRoundtrip(t *testing.T) {
	store := newTestTotpStore(t)
	if err := store.Save("admin", "SECRET123"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	enabled, err := store.Enabled("admin")
	if err != nil || !enabled {
		t.Fatalf("expected enabled after Save, enabled=%v err=%v", enabled, err)
	}
	secret, ok, err := store.Secret("admin")
	if err != nil || !ok || secret != "SECRET123" {
		t.Fatalf("expected secret roundtrip, secret=%q ok=%v err=%v", secret, ok, err)
	}
}

func TestTotpStorePersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2fa.json")
	if err := NewTotpStore(path).Save("admin", "SECRET123"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	store := NewTotpStore(path)
	enabled, err := store.Enabled("admin")
	if err != nil || !enabled {
		t.Fatalf("expected enabled from disk, enabled=%v err=%v", enabled, err)
	}
}

func TestTotpStoreDisableRemovesEntry(t *testing.T) {
	store := newTestTotpStore(t)
	if err := store.Save("admin", "SECRET123"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if err := store.Disable("admin"); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	enabled, err := store.Enabled("admin")
	if err != nil || enabled {
		t.Fatalf("expected disabled after Disable, enabled=%v err=%v", enabled, err)
	}
	_, ok, err := store.Secret("admin")
	if err != nil || ok {
		t.Fatalf("expected no secret after Disable, ok=%v err=%v", ok, err)
	}
}

func TestTotpStoreDisableMissingFile(t *testing.T) {
	store := newTestTotpStore(t)
	if err := store.Disable("admin"); err != nil {
		t.Fatalf("Disable() on missing file should not error, got %v", err)
	}
}