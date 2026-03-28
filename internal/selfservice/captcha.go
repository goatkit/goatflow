package selfservice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// VerifyCAPTCHA validates a CAPTCHA response token against the provider.
// Returns nil if CAPTCHA is disabled or verification passes.
func VerifyCAPTCHA(cfg *CAPTCHAConfig, responseToken string) error {
	if cfg == nil || cfg.Provider == "" || cfg.SecretKey == "" {
		return nil // CAPTCHA disabled.
	}

	switch cfg.Provider {
	case CAPTCHARecaptcha:
		return verifyRecaptchaV3(cfg.SecretKey, responseToken, cfg.Threshold)
	case CAPTCHAHCaptcha:
		return verifyHCaptcha(cfg.SecretKey, responseToken)
	default:
		return fmt.Errorf("unsupported CAPTCHA provider: %s", cfg.Provider)
	}
}

func verifyRecaptchaV3(secretKey, responseToken string, threshold float64) error {
	if threshold <= 0 {
		threshold = 0.5
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).PostForm(
		"https://www.google.com/recaptcha/api/siteverify",
		url.Values{
			"secret":   {secretKey},
			"response": {responseToken},
		},
	)
	if err != nil {
		return fmt.Errorf("recaptcha verify request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("recaptcha read response: %w", err)
	}

	var result struct {
		Success bool    `json:"success"`
		Score   float64 `json:"score"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("recaptcha parse response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("reCAPTCHA verification failed")
	}
	if result.Score < threshold {
		return fmt.Errorf("reCAPTCHA score %.2f below threshold %.2f", result.Score, threshold)
	}
	return nil
}

func verifyHCaptcha(secretKey, responseToken string) error {
	resp, err := (&http.Client{Timeout: 10 * time.Second}).PostForm(
		"https://hcaptcha.com/siteverify",
		url.Values{
			"secret":   {secretKey},
			"response": {responseToken},
		},
	)
	if err != nil {
		return fmt.Errorf("hcaptcha verify request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("hcaptcha read response: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("hcaptcha parse response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("hCaptcha verification failed")
	}
	return nil
}
