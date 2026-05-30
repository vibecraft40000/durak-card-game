package main

import (
	"net/http/httptest"
	"os"
	"testing"
)

func TestAdminActorFromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/admin/users", nil)
	req.Header.Set("X-Admin-Actor", "  alice\tadmin \n 42  ")

	actor := adminActorFromRequest(req)
	if actor != "alice admin 42" {
		t.Fatalf("unexpected actor value: %q", actor)
	}
}

func TestAdminActorFromRequestMissing(t *testing.T) {
	req := httptest.NewRequest("GET", "/admin/users", nil)
	if actor := adminActorFromRequest(req); actor != "" {
		t.Fatalf("expected empty actor, got %q", actor)
	}
}

func TestLoadDotEnvForLocalRuntimeSkipsWhenEnvIsStaging(t *testing.T) {
	t.Setenv("ENV", "staging")
	if err := loadDotEnvForLocalRuntime(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := os.LookupEnv("ENV"); !ok {
		t.Fatal("expected explicit ENV to remain set")
	}
}
