package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestWrapConnectErrorPasswordHint(t *testing.T) {
	err := wrapConnectError("ping", &pgconn.PgError{Code: "28P01", Message: "password authentication failed"})
	if err == nil {
		t.Fatal("expected error")
	}
	got := err.Error()
	if !strings.Contains(got, "AUTOSECRETS_DB_PASSWORD") {
		t.Fatalf("missing password-volume hint: %s", got)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "28P01" {
		t.Fatalf("expected wrapped 28P01, got %v", err)
	}
}

func TestWrapConnectErrorOtherCodes(t *testing.T) {
	err := wrapConnectError("ping", errors.New("connection refused"))
	if strings.Contains(err.Error(), "AUTOSECRETS_DB_PASSWORD") {
		t.Fatalf("unexpected hint: %s", err)
	}
}
