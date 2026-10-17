package notify

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"sovereign/internal/store"
)

const (
	DefaultSMTPTimeout = 5 * time.Second
)

// EmailClient delivers consolidated digests and test messages via SMTP with STARTTLS support.
type EmailClient struct {
	timeout time.Duration
}

// NewEmailClient creates a new SMTP email client with an enforced 5s timeout.
func NewEmailClient() *EmailClient {
	return &EmailClient{
		timeout: DefaultSMTPTimeout,
	}
}

// SMTPConfig holds SMTP connection parameters.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// DeadlineDigestItem represents an obligation entry in the email digest.
type DeadlineDigestItem struct {
	ID            string
	DocumentID    string
	Title         string
	DueDate       string
	DaysRemaining int
	Amount        *float64
	Currency      string
	URL           string
	Snippet       string
}

// SendDigest delivers an HTML and plain-text digest of impending deadlines to recipient.
func (c *EmailClient) SendDigest(ctx context.Context, cfg SMTPConfig, to string, items []DeadlineDigestItem) error {
	if len(items) == 0 {
		return nil
	}

	subject := fmt.Sprintf("Upcoming Deadlines & Bills (%d) — Sovereign Alert", len(items))
	if len(items) == 1 {
		item := items[0]
		subject = fmt.Sprintf("Upcoming Deadline: %s — Sovereign Alert", item.Title)
	}

	plainText := buildPlainTextDigest(items)
	htmlText := buildHTMLDigest(items)

	return c.sendMail(ctx, cfg, to, subject, plainText, htmlText)
}

// SendTestEmail transmits a test notification verifying SMTP credentials and host connectivity.
func (c *EmailClient) SendTestEmail(ctx context.Context, cfg SMTPConfig, to string) error {
	subject := "Test Notification — Sovereign Home Server"
	plainText := "This is a test notification from your Sovereign Home Server.\n\n" +
		"Your email alert configuration is functioning properly and ready to deliver document deadline digests."

	htmlText := `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Sovereign Alert Test</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #F8F9FA; margin: 0; padding: 24px; color: #0F172A;">
  <div style="max-width: 580px; margin: 0 auto; background: #FFFFFF; border: 1px solid #E5E7EB; border-radius: 8px; padding: 32px;">
    <h2 style="margin-top: 0; font-size: 20px; font-weight: 600; color: #0F172A;">Test Notification</h2>
    <p style="font-size: 14px; line-height: 1.6; color: #475569;">
      This is a test notification from your Sovereign Home Server.
    </p>
    <div style="background: #ECFDF5; border: 1px solid #A7F3D0; border-radius: 6px; padding: 16px; margin: 24px 0;">
      <p style="margin: 0; font-size: 14px; font-weight: 500; color: #065F46;">
        ● Email alert dispatch is operational.
      </p>
    </div>
    <p style="font-size: 13px; color: #64748B; margin-top: 24px; border-top: 1px solid #E5E7EB; padding-top: 16px;">
      100% Local &amp; Private: Your files and personal data never leave your home.
    </p>
  </div>
</body>
</html>`

	return c.sendMail(ctx, cfg, to, subject, plainText, htmlText)
}

// SendRollbackAlert delivers an email notifying the homeowner of an automated service rollback.
func (c *EmailClient) SendRollbackAlert(ctx context.Context, cfg SMTPConfig, to string, serviceName, failedVersion, restoredVersion, reason string) error {
	subject := fmt.Sprintf("Automated Rollback: %s — Sovereign Alert", serviceName)
	plainText := fmt.Sprintf("Automated Service Rollback Notification\n"+
		"=======================================\n\n"+
		"Update couldn't start properly. We've safely restored your previous working version.\n\n"+
		"Service: %s\n"+
		"Restored Working Version: %s\n"+
		"Failed Version: %s\n"+
		"Reason: %s\n\n"+
		"Your files, media, and documents remain secure and untouched.\n"+
		"---------------------------------------\n"+
		"100%% Local & Private: Your files and personal data never leave your home.\n",
		serviceName, restoredVersion, failedVersion, reason)

	htmlText := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Automated Service Rollback</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #F8F9FA; margin: 0; padding: 24px; color: #0F172A;">
  <div style="max-width: 580px; margin: 0 auto; background: #FFFFFF; border: 1px solid #E5E7EB; border-radius: 8px; padding: 32px;">
    <h2 style="margin-top: 0; font-size: 20px; font-weight: 600; color: #0F172A;">Automated Service Rollback</h2>
    <div style="background: #FFFBEB; border: 1px solid #FDE68A; border-radius: 6px; padding: 16px; margin: 20px 0;">
      <p style="margin: 0; font-size: 14px; font-weight: 500; color: #92400E;">
        ◆ Update couldn't start properly. We've safely restored your previous working version.
      </p>
    </div>
    <table style="width: 100%%; font-size: 14px; border-collapse: collapse; margin-top: 16px;">
      <tr>
        <td style="padding: 8px 0; color: #64748B; width: 140px;">Service:</td>
        <td style="padding: 8px 0; color: #0F172A; font-weight: 600;">%s</td>
      </tr>
      <tr>
        <td style="padding: 8px 0; color: #64748B;">Restored Version:</td>
        <td style="padding: 8px 0; color: #065F46; font-weight: 600;">%s</td>
      </tr>
      <tr>
        <td style="padding: 8px 0; color: #64748B;">Failed Update:</td>
        <td style="padding: 8px 0; color: #991B1B;">%s</td>
      </tr>
      <tr>
        <td style="padding: 8px 0; color: #64748B;">Reason:</td>
        <td style="padding: 8px 0; color: #475569;">%s</td>
      </tr>
    </table>
    <p style="font-size: 13px; color: #475569; margin-top: 20px;">
      Your media libraries, files, and documents are preserved and were not modified.
    </p>
    <p style="font-size: 13px; color: #64748B; margin-top: 24px; border-top: 1px solid #E5E7EB; padding-top: 16px;">
      100%% Local &amp; Private: Your files and personal data never leave your home.
    </p>
  </div>
</body>
</html>`, html.EscapeString(serviceName), html.EscapeString(restoredVersion), html.EscapeString(failedVersion), html.EscapeString(reason))

	return c.sendMail(ctx, cfg, to, subject, plainText, htmlText)
}

func (c *EmailClient) sendMail(ctx context.Context, cfg SMTPConfig, to, subject, plainBody, htmlBody string) error {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return errors.New("SMTP host cannot be empty")
	}
	port := cfg.Port
	if port <= 0 {
		port = DefaultSMTPPort
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return errors.New("recipient email cannot be empty")
	}
	from := strings.TrimSpace(cfg.From)
	if from == "" {
		from = DefaultSMTPFrom
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	boundary := fmt.Sprintf("boundary_sovereign_%d", time.Now().UnixNano())

	// Sanitize subject and encode per RFC 2047 if non-ASCII
	sanitizedSubject := strings.ReplaceAll(strings.ReplaceAll(subject, "\r", ""), "\n", " ")
	encodedSubject := mime.QEncoding.Encode("utf-8", sanitizedSubject)

	// Build MIME payload
	now := time.Now()
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", now.Format(time.RFC1123Z)))
	msg.WriteString(fmt.Sprintf("Message-ID: <%d.%s@sovereign.local>\r\n", now.UnixNano(), store.GenerateUUIDv4()))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary))

	// Plain text section
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msg.WriteString(plainBody)
	msg.WriteString("\r\n\r\n")

	// HTML section
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msg.WriteString(htmlBody)
	msg.WriteString("\r\n\r\n")

	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	dialer := &net.Dialer{Timeout: c.timeout}

	var conn net.Conn
	var err error

	if port == 465 {
		// SMTPS direct TLS
		tlsConfig := &tls.Config{ServerName: host}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("connection timed out while reaching SMTP mail server")
		}
		return fmt.Errorf("failed to connect to SMTP server at %s: %s", addr, sanitizeNetworkError(err))
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("failed to initialize SMTP handshake: %s", sanitizeNetworkError(err))
	}
	defer client.Quit()

	_ = client.Hello("sovereign.local")

	// Upgrade via STARTTLS if supported and not already TLS
	if port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{ServerName: host}
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("failed to negotiate TLS with SMTP server: %s", sanitizeNetworkError(err))
			}
		}
	}

	// Authenticate if credentials are provided
	if cfg.Username != "" && cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, host)
		if err := client.Auth(auth); err != nil {
			return errors.New("SMTP authentication failed: invalid username or password")
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL command failed: %s", sanitizeNetworkError(err))
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT command failed: %s", sanitizeNetworkError(err))
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA command failed: %s", sanitizeNetworkError(err))
	}

	if _, err := wc.Write([]byte(msg.String())); err != nil {
		_ = wc.Close()
		return fmt.Errorf("failed to transmit email message data: %s", sanitizeNetworkError(err))
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to finalize email delivery: %s", sanitizeNetworkError(err))
	}

	return nil
}

func buildPlainTextDigest(items []DeadlineDigestItem) string {
	var sb strings.Builder
	sb.WriteString("Upcoming Household Deadlines & Obligations\n")
	sb.WriteString("==========================================\n\n")

	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Title))
		daysText := fmt.Sprintf("%d days remaining", item.DaysRemaining)
		if item.DaysRemaining < 0 {
			daysText = fmt.Sprintf("%d days overdue", -item.DaysRemaining)
		} else if item.DaysRemaining == 0 {
			daysText = "due today"
		} else if item.DaysRemaining == 1 {
			daysText = "due tomorrow"
		}
		sb.WriteString(fmt.Sprintf("   Due: %s (%s)\n", item.DueDate, daysText))
		if item.Amount != nil {
			cur := item.Currency
			if cur == "" {
				cur = "USD"
			}
			sb.WriteString(fmt.Sprintf("   Amount: %.2f %s\n", *item.Amount, cur))
		}
		if item.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   Details: %s\n", item.Snippet))
		}
		if item.URL != "" {
			sb.WriteString(fmt.Sprintf("   Document: %s\n", item.URL))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("------------------------------------------\n")
	sb.WriteString("100% Local & Private: Your files and personal data never leave your home.\n")
	return sb.String()
}

func buildHTMLDigest(items []DeadlineDigestItem) string {
	var rows strings.Builder

	for _, item := range items {
		daysBadgeColor := "#3B82F6"
		daysBadgeBg := "#EFF6FF"
		if item.DaysRemaining < 0 {
			daysBadgeColor = "#991B1B"
			daysBadgeBg = "#FEE2E2"
		} else if item.DaysRemaining <= 3 {
			daysBadgeColor = "#EF4444"
			daysBadgeBg = "#FEF2F2"
		} else if item.DaysRemaining <= 7 {
			daysBadgeColor = "#F59E0B"
			daysBadgeBg = "#FFFBEB"
		}

		daysText := fmt.Sprintf("in %d days", item.DaysRemaining)
		if item.DaysRemaining < 0 {
			daysText = fmt.Sprintf("%d days overdue", -item.DaysRemaining)
		} else if item.DaysRemaining == 0 {
			daysText = "Today"
		} else if item.DaysRemaining == 1 {
			daysText = "Tomorrow"
		}

		amountStr := "—"
		if item.Amount != nil {
			cur := item.Currency
			if cur == "" {
				cur = "USD"
			}
			amountStr = html.EscapeString(fmt.Sprintf("%.2f %s", *item.Amount, cur))
		}

		snippetHTML := ""
		if item.Snippet != "" {
			snippetHTML = fmt.Sprintf(`<p style="margin: 4px 0 0 0; font-size: 12px; color: #64748B; font-style: italic;">%s</p>`, html.EscapeString(item.Snippet))
		}

		urlLink := item.URL
		if urlLink == "" {
			urlLink = "http://sovereign.local/docs"
		}
		escapedURL := html.EscapeString(urlLink)
		escapedTitle := html.EscapeString(item.Title)
		escapedDueDate := html.EscapeString(item.DueDate)

		row := fmt.Sprintf(`
		<tr style="border-bottom: 1px solid #E5E7EB;">
			<td style="padding: 16px 12px; vertical-align: top;">
				<strong style="font-size: 14px; color: #0F172A;">%s</strong>
				%s
			</td>
			<td style="padding: 16px 12px; vertical-align: top; white-space: nowrap;">
				<div style="font-size: 13px; color: #0F172A;">%s</div>
				<span style="display: inline-block; margin-top: 4px; padding: 2px 8px; border-radius: 9999px; font-size: 11px; font-weight: 500; background: %s; color: %s;">
					%s
				</span>
			</td>
			<td style="padding: 16px 12px; vertical-align: top; font-size: 14px; font-weight: 600; color: #0F172A; white-space: nowrap;">
				%s
			</td>
			<td style="padding: 16px 12px; vertical-align: top; text-align: right; white-space: nowrap;">
				<a href="%s" style="display: inline-block; background: #0F172A; color: #FFFFFF; text-decoration: none; padding: 6px 12px; border-radius: 6px; font-size: 12px; font-weight: 500;">
					View Document
				</a>
			</td>
		</tr>`, escapedTitle, snippetHTML, escapedDueDate, daysBadgeBg, daysBadgeColor, daysText, amountStr, escapedURL)

		rows.WriteString(row)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Sovereign Deadlines Digest</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #F8F9FA; margin: 0; padding: 24px; color: #0F172A;">
  <div style="max-width: 680px; margin: 0 auto; background: #FFFFFF; border: 1px solid #E5E7EB; border-radius: 8px; padding: 32px;">
    <div style="margin-bottom: 24px;">
      <h2 style="margin: 0; font-size: 20px; font-weight: 600; color: #0F172A;">Upcoming Household Deadlines</h2>
      <p style="margin: 6px 0 0 0; font-size: 14px; color: #64748B;">
        Here is your summary of impending contractual and payment obligations.
      </p>
    </div>

    <table style="width: 100%%; border-collapse: collapse; text-align: left;">
      <thead>
        <tr style="border-bottom: 2px solid #E5E7EB; font-size: 12px; color: #64748B; text-transform: uppercase;">
          <th style="padding: 8px 12px;">Document</th>
          <th style="padding: 8px 12px;">Due Date</th>
          <th style="padding: 8px 12px;">Amount</th>
          <th style="padding: 8px 12px; text-align: right;">Action</th>
        </tr>
      </thead>
      <tbody>
        %s
      </tbody>
    </table>

    <div style="margin-top: 32px; padding-top: 16px; border-top: 1px solid #E5E7EB; font-size: 12px; color: #64748B; text-align: center;">
      100%% Local &amp; Private: Your files and personal data never leave your home.
    </div>
  </div>
</body>
</html>`, rows.String())
}

// Ensure store.DocumentDeadline can be referenced cleanly if needed
var _ = store.DeadlineStatusPending
