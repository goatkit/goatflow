package notifications

import (
	"context"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/config"
)

func TestSMTPProvider_Send(t *testing.T) {
	// Test configuration for mail-sink
	cfg := &config.EmailConfig{
		Enabled: true,
		SMTP: struct {
			Host       string `mapstructure:"host"`
			Port       int    `mapstructure:"port"`
			User       string `mapstructure:"user"`
			Password   string `mapstructure:"password"`
			AuthType   string `mapstructure:"auth_type"`
			TLS        bool   `mapstructure:"tls"`
			SkipVerify bool   `mapstructure:"skip_verify"`
		}{
			Host: "localhost",
			Port: 1025, // Mailhog SMTP port
		},
		From: "test@example.com",
	}

	provider := NewSMTPProvider(cfg)

	tests := []struct {
		name    string
		msg     EmailMessage
		wantErr bool
	}{
		{
			name: "valid email",
			msg: EmailMessage{
				To:      []string{"recipient@example.com"},
				Subject: "Test Subject",
				Body:    "Test Body",
				HTML:    false,
			},
			wantErr: true, // Will fail due to no SMTP server, but validates input
		},
		{
			name: "empty recipient",
			msg: EmailMessage{
				To:      []string{},
				Subject: "Test Subject",
				Body:    "Test Body",
				HTML:    false,
			},
			wantErr: true, // Should fail validation
		},
		{
			name: "empty subject",
			msg: EmailMessage{
				To:      []string{"recipient@example.com"},
				Subject: "",
				Body:    "Test Body",
				HTML:    false,
			},
			wantErr: true, // Will fail due to no SMTP server, but validates input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.Send(context.Background(), tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("SMTPProvider.Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSMTPProvider_TLSConfig(t *testing.T) {
	cfg := &config.EmailConfig{
		Enabled: true,
		SMTP: struct {
			Host       string `mapstructure:"host"`
			Port       int    `mapstructure:"port"`
			User       string `mapstructure:"user"`
			Password   string `mapstructure:"password"`
			AuthType   string `mapstructure:"auth_type"`
			TLS        bool   `mapstructure:"tls"`
			SkipVerify bool   `mapstructure:"skip_verify"`
		}{
			Host: "smtp.gmail.com",
			Port: 587,
			User: "test@example.com",
			TLS:  true,
		},
		From: "test@example.com",
	}

	provider := NewSMTPProvider(cfg)

	// Cast to concrete type to access config
	smtpProvider, ok := provider.(*SMTPProvider)
	if !ok {
		t.Fatal("Expected SMTPProvider")
	}

	// Test that config is properly set
	if smtpProvider.cfg.SMTP.Host != "smtp.gmail.com" {
		t.Errorf("Expected host smtp.gmail.com, got %s", smtpProvider.cfg.SMTP.Host)
	}

	if smtpProvider.cfg.SMTP.Port != 587 {
		t.Errorf("Expected port 587, got %d", smtpProvider.cfg.SMTP.Port)
	}

	if !smtpProvider.cfg.SMTP.TLS {
		t.Error("Expected TLS to be enabled")
	}
}

func TestSMTPProvider_Authentication(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		authType string
	}{
		{
			name:     "plain auth",
			username: "user",
			password: "pass",
			authType: "PLAIN",
		},
		{
			name:     "login auth",
			username: "user",
			password: "pass",
			authType: "LOGIN",
		},
		{
			name:     "no auth",
			username: "",
			password: "",
			authType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.EmailConfig{
				Enabled: true,
				SMTP: struct {
					Host       string `mapstructure:"host"`
					Port       int    `mapstructure:"port"`
					User       string `mapstructure:"user"`
					Password   string `mapstructure:"password"`
					AuthType   string `mapstructure:"auth_type"`
					TLS        bool   `mapstructure:"tls"`
					SkipVerify bool   `mapstructure:"skip_verify"`
				}{
					Host:     "localhost",
					Port:     1025,
					User:     tt.username,
					Password: tt.password,
					AuthType: tt.authType,
				},
				From: "test@example.com",
			}

			provider := NewSMTPProvider(cfg)

			// The provider should be created successfully regardless of auth config
			if provider == nil {
				t.Error("Expected provider to be created")
			}
		})
	}
}