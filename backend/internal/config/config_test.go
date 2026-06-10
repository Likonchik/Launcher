package config

import "testing"

func TestValidateRejectsDevSecretsInProduction(t *testing.T) {
	cfg := Config{AppEnv: "production", JWTSecret: "dev-only-change-me"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("production с dev JWT-секретом должен отклоняться")
	}
	cfg.JWTSecret = "change-me-in-production"
	if err := cfg.Validate(); err == nil {
		t.Fatal("production с compose-заглушкой JWT-секрета должен отклоняться")
	}
}

func TestValidateAllowsDevSecretsInDevelopment(t *testing.T) {
	cfg := Config{AppEnv: "development", JWTSecret: "dev-only-change-me"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development должен работать с дефолтным секретом: %v", err)
	}
}

func TestValidateAllowsRealSecretInProduction(t *testing.T) {
	cfg := Config{AppEnv: "production", JWTSecret: "a-real-32-char-random-secret-value"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production с нормальным секретом должен проходить: %v", err)
	}
}
