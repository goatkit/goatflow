package notifications

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/config"
)

func startFakeSMTPServer(t *testing.T) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake SMTP server: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleFakeSMTPConnection(conn)
		}
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("failed to parse fake SMTP address: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		_ = ln.Close()
		t.Fatalf("failed to parse fake SMTP port: %v", err)
	}

	t.Cleanup(func() {
		_ = ln.Close()
	})

	return host, port
}

func handleFakeSMTPConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	write := func(msg string) {
		_, _ = writer.WriteString(msg)
		_ = writer.Flush()
	}

	write("220 localhost ESMTP\r\n")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			write("250-localhost\r\n250 OK\r\n")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			write("250 OK\r\n")
		case strings.HasPrefix(cmd, "RCPT TO"):
			write("250 OK\r\n")
		case strings.HasPrefix(cmd, "DATA"):
			write("354 End data with <CR><LF>.<CR><LF>\r\n")
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
			}
			write("250 OK\r\n")
		case strings.HasPrefix(cmd, "QUIT"):
			write("221 Bye\r\n")
			return
		default:
			write("250 OK\r\n")
		}
	}
}

func TestSMTPProvider_Send(t *testing.T) {
	host, port := startFakeSMTPServer(t)

	cfg := &config.EmailConfig{
		Enabled: true,
		SMTP: struct {
			Host       string `mapstructure:"host"`
			Port       int    `mapstructure:"port"`
			User       string `mapstructure:"user"`
			Password   string `mapstructure:"password"`
			AuthType   string `mapstructure:"auth_type"`
			TLS        bool   `mapstructure:"tls"`
			TLSMode    string `mapstructure:"tls_mode"`
			SkipVerify bool   `mapstructure:"skip_verify"`
		}{
			Host:    host,
			Port:    port,
			TLSMode: "",
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
			wantErr: false, // Accept success when local mail sink is available
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
			wantErr: false, // Accept success; subject validation occurs upstream
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
			TLSMode    string `mapstructure:"tls_mode"`
			SkipVerify bool   `mapstructure:"skip_verify"`
		}{
			Host:    "smtp.gmail.com",
			Port:    587,
			User:    "test@example.com",
			TLSMode: "",
			TLS:     true,
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
					TLSMode    string `mapstructure:"tls_mode"`
					SkipVerify bool   `mapstructure:"skip_verify"`
				}{
					Host:     "localhost",
					Port:     1025,
					User:     tt.username,
					TLSMode:  "",
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
