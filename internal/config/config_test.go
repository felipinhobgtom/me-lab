package config

import "testing"

func TestLoadDefaultsAndValidation(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Fatalf("expected port 9090, got %s", cfg.Port)
	}

	t.Setenv("PORT", "99999")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid PORT error")
	}

	t.Setenv("PORT", "8080")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("expected short JWT_SECRET error")
	}
}
