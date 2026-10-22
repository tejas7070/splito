package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"
	"tripledger/config"
)

// DispatchEmail sends an email using the best available channel:
// 1. Brevo HTTPS API (Port 443 - free 300/day, delivers to ANY recipient without domain verification)
// 2. Resend HTTPS API (Port 443 - free sandbox, strictly delivers to owner)
// 3. Local SMTP (Port 587 - for local development)
func DispatchEmail(cfg *config.Config, toEmail, subject, htmlContent, logContext string) {
	if cfg == nil {
		return
	}
	if cfg.BrevoAPIKey == "" && cfg.ResendAPIKey == "" && (cfg.SMTPHost == "" || cfg.SMTPUser == "") {
		fmt.Printf("[%s] No email provider configured (set BREVO_API_KEY in environment)\n", logContext)
		return
	}

	go func() {
		from := strings.TrimSpace(cfg.SMTPFrom)
		if from == "" {
			from = strings.TrimSpace(cfg.SMTPUser)
		}
		if from == "" {
			from = "tejasulawekar@gmail.com"
		}

		// 1. Try Brevo HTTP API (Port 443 - works on Render Free, sends to ANY recipient without custom domain)
		cleanBrevoKey := strings.TrimSpace(cfg.BrevoAPIKey)
		if cleanBrevoKey != "" {
			brevoSender := from
			if strings.Contains(brevoSender, "resend.dev") || !strings.Contains(brevoSender, "@") {
				brevoSender = "tejasulawekar@gmail.com"
			}

			brevoPayload := map[string]interface{}{
				"sender": map[string]string{
					"name":  "Splito",
					"email": brevoSender,
				},
				"to": []map[string]string{
					{"email": toEmail},
				},
				"subject":     subject,
				"htmlContent": htmlContent,
			}
			jsonBytes, err := json.Marshal(brevoPayload)
			if err == nil {
				req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(jsonBytes))
				if err == nil {
					req.Header.Set("accept", "application/json")
					req.Header.Set("api-key", cleanBrevoKey)
					req.Header.Set("Content-Type", "application/json")
					client := &http.Client{Timeout: 10 * time.Second}
					resp, err := client.Do(req)
					if err == nil {
						bodyBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							fmt.Printf("[%s SUCCESS] Dispatched email to %s via Brevo API\n", logContext, toEmail)
							return
						}
						fmt.Printf("[%s BREVO ERROR] HTTP %d: %s\n", logContext, resp.StatusCode, string(bodyBytes))
					} else {
						fmt.Printf("[%s BREVO ERROR] %v\n", logContext, err)
					}
				}
			}
		}

		// 2. Try Resend HTTP API (Port 443 - free sandbox strictly delivers to account owner)
		cleanResendKey := strings.TrimSpace(cfg.ResendAPIKey)
		if cleanResendKey != "" {
			sender := "Splito <onboarding@resend.dev>"
			cleanFrom := strings.ToLower(from)
			if cleanFrom != "" && strings.Contains(cleanFrom, "@") && !strings.HasSuffix(cleanFrom, "@gmail.com") && !strings.HasSuffix(cleanFrom, "@yahoo.com") && !strings.Contains(cleanFrom, "resend.dev") {
				sender = from
			}
			resendPayload := map[string]interface{}{
				"from":    sender,
				"to":      []string{toEmail},
				"subject": subject,
				"html":    htmlContent,
			}
			jsonBytes, err := json.Marshal(resendPayload)
			if err == nil {
				req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonBytes))
				if err == nil {
					req.Header.Set("Authorization", "Bearer "+cleanResendKey)
					req.Header.Set("Content-Type", "application/json")
					client := &http.Client{Timeout: 10 * time.Second}
					resp, err := client.Do(req)
					if err == nil {
						bodyBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							fmt.Printf("[%s SUCCESS] Dispatched email to %s via Resend API\n", logContext, toEmail)
							return
						}
						fmt.Printf("[%s RESEND ERROR] HTTP %d: %s\n", logContext, resp.StatusCode, string(bodyBytes))
					} else {
						fmt.Printf("[%s RESEND ERROR] %v\n", logContext, err)
					}
				}
			}
		}

		// 3. Fallback to standard SMTP (Port 587 - works on localhost)
		if cfg.SMTPHost != "" && cfg.SMTPUser != "" {
			headers := fmt.Sprintf("MIME-Version: 1.0\r\n"+
				"From: Splito <%s>\r\n"+
				"To: %s\r\n"+
				"Subject: %s\r\n"+
				"Content-Type: text/html; charset=UTF-8\r\n\r\n", from, toEmail, subject)
			msg := []byte(headers + htmlContent)
			auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
			addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
			if err := smtp.SendMail(addr, auth, from, []string{toEmail}, msg); err != nil {
				fmt.Printf("[%s SMTP ERROR] Failed to send email to %s: %v\n", logContext, toEmail, err)
			} else {
				fmt.Printf("[%s SMTP SUCCESS] Email dispatched to %s\n", logContext, toEmail)
			}
		}
	}()
}
