package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCMemberValue  string
	OIDCAdminValue   string
	SessionSecret    string
	SMTPAddr         string
	SMTPUsername     string
	SMTPPassword     string
	SMTPFrom         string
	PublicBaseURL    string
	ArchiveGraceDays int
	LogLevel         string
}

// Load reads configuration from environment variables.
func Load() Config {
	graceDays := 7
	if v := os.Getenv("CONFERO_ARCHIVE_GRACE_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			graceDays = n
		}
	}

	logLevel := "info"
	if v := os.Getenv("CONFERO_LOG_LEVEL"); v != "" {
		logLevel = v
	}

	httpAddr := ":8080"
	if v := os.Getenv("CONFERO_HTTP_ADDR"); v != "" {
		httpAddr = v
	}

	return Config{
		HTTPAddr:         httpAddr,
		DatabaseURL:      os.Getenv("CONFERO_DATABASE_URL"),
		OIDCIssuerURL:    os.Getenv("CONFERO_OIDC_ISSUER_URL"),
		OIDCClientID:     os.Getenv("CONFERO_OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("CONFERO_OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:  os.Getenv("CONFERO_OIDC_REDIRECT_URL"),
		OIDCMemberValue:  os.Getenv("CONFERO_OIDC_MEMBER_VALUE"),
		OIDCAdminValue:   os.Getenv("CONFERO_OIDC_ADMIN_VALUE"),
		SessionSecret:    os.Getenv("CONFERO_SESSION_SECRET"),
		SMTPAddr:         os.Getenv("CONFERO_SMTP_ADDR"),
		SMTPUsername:     os.Getenv("CONFERO_SMTP_USERNAME"),
		SMTPPassword:     os.Getenv("CONFERO_SMTP_PASSWORD"),
		SMTPFrom:         os.Getenv("CONFERO_SMTP_FROM"),
		PublicBaseURL:    os.Getenv("CONFERO_PUBLIC_BASE_URL"),
		ArchiveGraceDays: graceDays,
		LogLevel:         logLevel,
	}
}

// Validate returns an error if any required field is missing.
func (c Config) Validate() error {
	var errs []error
	required := []struct {
		val, name string
	}{
		{c.DatabaseURL, "CONFERO_DATABASE_URL"},
		{c.OIDCIssuerURL, "CONFERO_OIDC_ISSUER_URL"},
		{c.OIDCClientID, "CONFERO_OIDC_CLIENT_ID"},
		{c.OIDCClientSecret, "CONFERO_OIDC_CLIENT_SECRET"},
		{c.OIDCRedirectURL, "CONFERO_OIDC_REDIRECT_URL"},
		{c.OIDCMemberValue, "CONFERO_OIDC_MEMBER_VALUE"},
		{c.OIDCAdminValue, "CONFERO_OIDC_ADMIN_VALUE"},
		{c.SessionSecret, "CONFERO_SESSION_SECRET"},
	}
	for _, r := range required {
		if r.val == "" {
			errs = append(errs, errors.New("required env var not set: "+r.name))
		}
	}
	if len(c.SessionSecret) < 32 {
		errs = append(errs, errors.New("CONFERO_SESSION_SECRET must be at least 32 bytes"))
	}
	return errors.Join(errs...)
}
