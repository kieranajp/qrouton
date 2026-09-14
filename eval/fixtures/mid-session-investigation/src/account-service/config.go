package account

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultRetryDeadline = 2 * time.Second
	defaultAttempts      = 3
)

type Config struct {
	RetryDeadline time.Duration
	Attempts      int
	LedgerTimeout time.Duration
	Endpoint      string
}

func LoadConfig() Config {
	return Config{
		RetryDeadline: durationEnv("ACCOUNT_RETRY_DEADLINE_MS", defaultRetryDeadline),
		Attempts:      intEnv("ACCOUNT_RETRY_ATTEMPTS", defaultAttempts),
		LedgerTimeout: durationEnv("ACCOUNT_LEDGER_TIMEOUT_MS", 5*time.Second),
		Endpoint:      os.Getenv("ACCOUNT_UPSTREAM"),
	}
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	milliseconds, err := strconv.Atoi(raw)
	if err != nil || milliseconds <= 0 {
		return fallback
	}
	return time.Duration(milliseconds) * time.Millisecond
}

func intEnv(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
