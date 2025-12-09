package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

type IPVersion int

const (
	IPv4 IPVersion = iota
	IPv6
	IPAny
)

func (v IPVersion) Network() string {
	switch v {
	case IPAny:
		return "tcp"
	case IPv4:
		return "tcp4"
	case IPv6:
		return "tcp6"
	default:
		panic("invalid option!")
	}
}

type SMTPSecurity int

const (
	SMTPSecure SMTPSecurity = iota
	SMTPInsecureDangerous
)

func (v SMTPSecurity) EnableSecurity() bool {
	switch v {
	case SMTPSecure:
		return true
	case SMTPInsecureDangerous:
		return false
	default:
		return true
	}
}

type smtpConfig struct {
	// Can be empty string, in which case email is used
	senderName  string
	senderEmail string
	// Used for the Message-ID
	domain     string
	serverHost string
	serverPort string
	ipVersion  IPVersion
	// Can be nil, in which case no authentication is performed
	auth     smtp.Auth
	security SMTPSecurity
	// Disable keepAlive, if unset defaults to false (keepAlive enabled)
	disableKeepAlive bool
	// Path to email templates directory (optional)
	templatesPath string
}

type smtpActionsEmailSender struct {
	client           *smtp.Client
	config           *smtpConfig
	templates        *template.Template
	tokenBroadcaster *TokenBroadcaster
	m                sync.Mutex
	stopChan         chan bool
	errChan          chan error
}

func loadEmailTemplates(templatesPath string) (*template.Template, error) {
	if templatesPath == "" {
		return nil, nil
	}

	// Check if directory exists
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("templates directory does not exist: %s", templatesPath)
	}

	tmpl := template.New("")

	// Find all .txt and .html files in the templates directory
	pattern := filepath.Join(templatesPath, "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list template files: %v", err)
	}

	for _, file := range files {
		ext := filepath.Ext(file)
		if ext == ".txt" || ext == ".html" {
			content, err := os.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("failed to read template file %s: %v", file, err)
			}

			// Use the base name (without extension) as the template name
			baseName := filepath.Base(file)
			_, err = tmpl.New(baseName).Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("failed to parse template %s: %v", file, err)
			}
		}
	}

	return tmpl, nil
}

func createConnectedSmtpClient(config *smtpConfig) (*smtp.Client, error) {
	serverAddr := config.serverHost + ":" + config.serverPort
	// We don't use SMTP dial because then the local name is set to "localhost", which can lead to
	// issues when using e.g. IP authentication
	conn, err := net.Dial(config.ipVersion.Network(), serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %v", serverAddr, err)
	}

	client, err := smtp.NewClient(conn, config.serverHost)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SMTP client: %v", err)
	}

	// We set the localName based on the actual connection address, which is done using `client.Hello`
	localAddr := conn.LocalAddr().String()
	localName, _, _ := net.SplitHostPort(localAddr)
	err = client.Hello(localName)
	if err != nil {
		return nil, fmt.Errorf("Error sending EHLO: %v\n", err)
	}

	if config.security.EnableSecurity() {
		tlsConfig := &tls.Config{
			ServerName: config.serverHost,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to start TLS: %v", err)
		}
	} else {
		log.Println("[DANGER] TLS not enabled, messages are not secured and can be read when intercepted!")
	}

	if config.auth != nil {
		if err = client.Auth(config.auth); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to authenticate: %v", err)
		}
	}
	return client, nil
}

func generateMessageID(senderEmail, receiverEmail, body string, time string, domain string) string {
	input := fmt.Sprintf("%s|%s|%s|%s", senderEmail, receiverEmail, body, time)
	hash := sha256.Sum256([]byte(input))

	// Using just the first 32 is fine, this is not used for any security purposes
	hashStr := fmt.Sprintf("%x", hash)[:32]

	return fmt.Sprintf("<%s@%s>", hashStr, domain)
}

// generateBoundary creates a random boundary for multipart MIME messages
func generateBoundary() string {
	var buf [16]byte
	rand.Read(buf[:])
	return "boundary_" + hex.EncodeToString(buf[:])
}

// receiverName can be empty string, in which case the email is used.
// body is used for plain text, htmlBody is optional for HTML version
func (emailSender *smtpActionsEmailSender) SendEmail(receiverName string, receiverEmail string, subject string, body string) error {
	return emailSender.SendEmailWithHTML(receiverName, receiverEmail, subject, body, "")
}

// SendEmailWithHTML sends an email with both plain text and HTML parts
func (emailSender *smtpActionsEmailSender) SendEmailWithHTML(receiverName string, receiverEmail string, subject string, body string, htmlBody string) error {
	var fromHeader, toHeader string

	if emailSender.config.senderName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", emailSender.config.senderName, emailSender.config.senderEmail)
	} else {
		fromHeader = emailSender.config.senderEmail
	}

	if receiverName != "" {
		toHeader = fmt.Sprintf("%s <%s>", receiverName, receiverEmail)
	} else {
		toHeader = receiverEmail
	}

	date := time.Now().Format(time.RFC1123Z)
	messageId := generateMessageID(emailSender.config.senderEmail, receiverEmail, body, date, emailSender.config.domain)

	var message string

	if htmlBody != "" {
		// Create multipart/alternative message with both text and HTML
		boundary := generateBoundary()

		headers := []string{
			fmt.Sprintf("From: %s", fromHeader),
			fmt.Sprintf("To: %s", toHeader),
			fmt.Sprintf("Subject: %s", subject),
			fmt.Sprintf("Date: %s", date),
			fmt.Sprintf("Message-ID: %s", messageId),
			"MIME-Version: 1.0",
			fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"", boundary),
		}

		var buf bytes.Buffer
		buf.WriteString(strings.Join(headers, "\r\n"))
		buf.WriteString("\r\n\r\n")

		// Create multipart writer
		writer := multipart.NewWriter(&buf)
		writer.SetBoundary(boundary)

		// Add plain text part
		textHeader := textproto.MIMEHeader{}
		textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
		textPart, _ := writer.CreatePart(textHeader)
		textPart.Write([]byte(body))

		// Add HTML part
		htmlHeader := textproto.MIMEHeader{}
		htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
		htmlPart, _ := writer.CreatePart(htmlHeader)
		htmlPart.Write([]byte(htmlBody))

		writer.Close()
		message = buf.String()
	} else {
		// Simple plain text message
		headers := []string{
			fmt.Sprintf("From: %s", fromHeader),
			fmt.Sprintf("To: %s", toHeader),
			fmt.Sprintf("Subject: %s", subject),
			fmt.Sprintf("Date: %s", date),
			fmt.Sprintf("Message-ID: %s", messageId),
			"MIME-Version: 1.0",
			"Content-Type: text/plain; charset=UTF-8",
		}

		message = strings.Join(headers, "\r\n") + "\r\n\r\n" + body
	}

	emailSender.m.Lock()
	defer emailSender.m.Unlock()

	var mailErr error = nil
	for range 3 {
		if mailErr != nil {
			newClient, err := createConnectedSmtpClient(emailSender.config)
			if err != nil {
				return err
			}
			emailSender.client = newClient
		}

		if emailSender.client == nil {
			err := emailSender.Start(time.Minute * 5)
			if err != nil {
				return err
			}
		}

		err := emailSender.client.Mail(emailSender.config.senderEmail)
		if err != nil {
			mailErr = fmt.Errorf("failed to set sender: %v", err)
			continue
		}

		if err = emailSender.client.Rcpt(receiverEmail); err != nil {
			mailErr = fmt.Errorf("failed to set recipient: %v", err)
			continue
		}

		writer, err := emailSender.client.Data()
		if err != nil {
			mailErr = fmt.Errorf("failed to get data writer: %v", err)
			continue
		}

		_, err = writer.Write([]byte(message))
		if err != nil {
			mailErr = fmt.Errorf("failed to write message: %v", err)
			continue
		}

		err = writer.Close()
		if err != nil {
			mailErr = fmt.Errorf("failed to close writer: %v", err)
			continue
		}

		// If we reach here everything is successful, so reset any previous errors and break loop
		mailErr = nil
		break
	}

	return mailErr
}

func (emailSender *smtpActionsEmailSender) Close() error {
	emailSender.m.Lock()
	defer emailSender.m.Unlock()

	emailSender.StopKeepAlive()

	if emailSender.client != nil {
		return emailSender.client.Quit()
	}
	return nil
}

func makeGreeting(displayName string) string {
	if displayName != "" {
		return fmt.Sprintf("Dear %s,", displayName)
	} else {
		return "Hello,"
	}
}

// renderTemplate renders a template with the given data, returns empty string if template doesn't exist
func (emailSender *smtpActionsEmailSender) renderTemplate(templateName string, data any) (string, error) {
	if emailSender.templates == nil {
		return "", nil
	}

	tmpl := emailSender.templates.Lookup(templateName)
	if tmpl == nil {
		return "", nil
	}

	var buf bytes.Buffer
	err := tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to render template %s: %v", templateName, err)
	}

	return buf.String(), nil
}

func (emailSender *smtpActionsEmailSender) SendSignupEmailAddressVerificationCode(emailAddress string, emailAddressVerificationCode string) error {
	// Broadcast token for testing/automation
	if emailSender.tokenBroadcaster != nil {
		emailSender.tokenBroadcaster.BroadcastSignupVerification(emailAddress, emailAddressVerificationCode)
	}

	subject := "Signup verification code"

	data := map[string]any{
		"EmailAddress":     emailAddress,
		"VerificationCode": emailAddressVerificationCode,
	}

	// Try to render templates
	textBody, err := emailSender.renderTemplate("signup_verification.txt", data)
	if err != nil {
		return err
	}
	htmlBody, err := emailSender.renderTemplate("signup_verification.html", data)
	if err != nil {
		return err
	}

	// Fallback to hardcoded message if no templates found
	if textBody == "" {
		textBody = fmt.Sprintf("Your email address verification code is %s.", emailAddressVerificationCode)
	}

	return emailSender.SendEmailWithHTML("", emailAddress, subject, textBody, htmlBody)
}

func (emailSender *smtpActionsEmailSender) SendUserEmailAddressUpdateEmailVerificationCode(emailAddress string, displayName string, emailAddressVerificationCode string) error {
	// Broadcast token for testing/automation
	if emailSender.tokenBroadcaster != nil {
		emailSender.tokenBroadcaster.BroadcastEmailUpdateVerification(emailAddress, emailAddressVerificationCode)
	}

	subject := "Email update verification code"

	data := map[string]any{
		"EmailAddress":     emailAddress,
		"DisplayName":      displayName,
		"VerificationCode": emailAddressVerificationCode,
		"Greeting":         makeGreeting(displayName),
	}

	// Try to render templates
	textBody, err := emailSender.renderTemplate("email_update_verification.txt", data)
	if err != nil {
		return err
	}
	htmlBody, err := emailSender.renderTemplate("email_update_verification.html", data)
	if err != nil {
		return err
	}

	// Fallback to hardcoded message if no templates found
	if textBody == "" {
		greeting := makeGreeting(displayName)
		codeMessage := fmt.Sprintf("You have made a request to update your email. Your verification code is %s.", emailAddressVerificationCode)
		textBody = fmt.Sprintf("%s\n\n%s", greeting, codeMessage)
	}

	return emailSender.SendEmailWithHTML(displayName, emailAddress, subject, textBody, htmlBody)
}

func (emailSender *smtpActionsEmailSender) SendUserPasswordResetTemporaryPassword(emailAddress string, displayName string, temporaryPassword string) error {
	// Broadcast token for testing/automation
	if emailSender.tokenBroadcaster != nil {
		emailSender.tokenBroadcaster.BroadcastPasswordReset(emailAddress, temporaryPassword)
	}

	subject := "Password reset temporary password"

	data := map[string]any{
		"EmailAddress":      emailAddress,
		"DisplayName":       displayName,
		"TemporaryPassword": temporaryPassword,
		"Greeting":          makeGreeting(displayName),
	}

	// Try to render templates
	textBody, err := emailSender.renderTemplate("password_reset.txt", data)
	if err != nil {
		return err
	}
	htmlBody, err := emailSender.renderTemplate("password_reset.html", data)
	if err != nil {
		return err
	}

	// Fallback to hardcoded message if no templates found
	if textBody == "" {
		greeting := makeGreeting(displayName)
		passwordMessage := fmt.Sprintf("Your password reset temporary password is %s.", temporaryPassword)
		textBody = fmt.Sprintf("%s\n\n%s", greeting, passwordMessage)
	}

	return emailSender.SendEmailWithHTML(displayName, emailAddress, subject, textBody, htmlBody)
}

func (emailSender *smtpActionsEmailSender) SendUserSignedInNotification(emailAddress string, displayName string, time time.Time) error {
	subject := "Sign-in detected"

	data := map[string]any{
		"EmailAddress": emailAddress,
		"DisplayName":  displayName,
		"Time":         time.UTC().Format("January 2, 2006 15:04:05"),
		"Greeting":     makeGreeting(displayName),
	}

	// Try to render templates
	textBody, err := emailSender.renderTemplate("signin_notification.txt", data)
	if err != nil {
		return err
	}
	htmlBody, err := emailSender.renderTemplate("signin_notification.html", data)
	if err != nil {
		return err
	}

	// Fallback to hardcoded message if no templates found
	if textBody == "" {
		greeting := makeGreeting(displayName)
		notificationMessage := fmt.Sprintf("We detected a sign-in to your account at %s (UTC).", time.UTC().Format("January 2, 2006 15:04:05"))
		textBody = fmt.Sprintf("%s\n\n%s", greeting, notificationMessage)
	}

	return emailSender.SendEmailWithHTML(displayName, emailAddress, subject, textBody, htmlBody)
}

func (emailSender *smtpActionsEmailSender) SendUserPasswordUpdatedNotification(emailAddress string, displayName string, time time.Time) error {
	subject := "Password updated"

	data := map[string]any{
		"EmailAddress": emailAddress,
		"DisplayName":  displayName,
		"Time":         time.UTC().Format("January 2, 2006 15:04:05"),
		"Greeting":     makeGreeting(displayName),
	}

	// Try to render templates
	textBody, err := emailSender.renderTemplate("password_updated_notification.txt", data)
	if err != nil {
		return err
	}
	htmlBody, err := emailSender.renderTemplate("password_updated_notification.html", data)
	if err != nil {
		return err
	}

	// Fallback to hardcoded message if no templates found
	if textBody == "" {
		greeting := makeGreeting(displayName)
		notificationMessage := fmt.Sprintf("Your account password was updated at %s (UTC).", time.UTC().Format("January 2, 2006 15:04:05"))
		textBody = fmt.Sprintf("%s\n\n%s", greeting, notificationMessage)
	}

	return emailSender.SendEmailWithHTML(displayName, emailAddress, subject, textBody, htmlBody)
}

func (emailSender *smtpActionsEmailSender) SendUserEmailAddressUpdatedNotification(emailAddress string, displayName string, newEmailAddress string, time time.Time) error {
	subject := "Email updated"

	data := map[string]any{
		"EmailAddress":    emailAddress,
		"DisplayName":     displayName,
		"NewEmailAddress": newEmailAddress,
		"Time":            time.UTC().Format("January 2, 2006 15:04:05"),
		"Greeting":        makeGreeting(displayName),
	}

	// Try to render templates
	textBody, err := emailSender.renderTemplate("email_updated_notification.txt", data)
	if err != nil {
		return err
	}
	htmlBody, err := emailSender.renderTemplate("email_updated_notification.html", data)
	if err != nil {
		return err
	}

	// Fallback to hardcoded message if no templates found
	if textBody == "" {
		greeting := makeGreeting(displayName)
		notificationMessage := fmt.Sprintf("Your account email address was updated to %s at %s (UTC).", newEmailAddress, time.UTC().Format("January 2, 2006 15:04:05"))
		textBody = fmt.Sprintf("%s\n\n%s", greeting, notificationMessage)
	}

	return emailSender.SendEmailWithHTML(displayName, emailAddress, subject, textBody, htmlBody)
}

// NOTE: Mutex is not unlocked in case of error!
func (emailSender *smtpActionsEmailSender) KeepAlive() error {
	emailSender.m.Lock()

	// NOOP command keeps connection alive
	if err := emailSender.client.Noop(); err != nil {
		// We do not unlock so that caller can keep the lock to reestablish connection
		return fmt.Errorf("keep-alive failed: %v", err)
	}

	emailSender.m.Unlock()
	return nil
}

// Only call this if the emailSender is locked!
func (emailSender *smtpActionsEmailSender) Start(interval time.Duration) error {
	emailSender.errChan = make(chan error, 1)
	emailSender.stopChan = make(chan bool)

	newClient, err := createConnectedSmtpClient(emailSender.config)
	if err != nil {
		return fmt.Errorf("Could not start emailSender: %v\n", err)
	}
	emailSender.client = newClient

	if emailSender.config.disableKeepAlive {
		// Do nothing, since we disable keepalive
	} else {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if err := emailSender.KeepAlive(); err != nil {
						log.Println("Keep-alive failed, reestablishing connection...")
						newClient, err := createConnectedSmtpClient(emailSender.config)
						if err != nil {
							emailSender.errChan <- fmt.Errorf("could not reestablish SMTP connection: %v", err)
							return
						}
						emailSender.client = newClient
						emailSender.m.Unlock()
					} else {
						// We're already unlocked in this case
						log.Println("Keep-alive sent successfully")
					}
				case <-emailSender.stopChan:
					log.Println("Keep-alive routine stopped")
					return
				}
			}
		}()
	}

	return nil
}

func (emailSender *smtpActionsEmailSender) StopKeepAlive() {
	if emailSender.stopChan != nil {
		select {
		case emailSender.stopChan <- true:
		default:
			// Channel might be closed already
		}
		close(emailSender.stopChan)
	}
}
