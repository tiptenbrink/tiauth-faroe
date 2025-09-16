package main

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"sync"
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
}

type smtpActionsEmailSender struct {
	client   *smtp.Client
	config   *smtpConfig
	m        sync.Mutex
	stopChan chan bool
	errChan  chan error
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

// receiverName can be empty string, in which case the email is used.
func (emailSender *smtpActionsEmailSender) SendEmail(receiverName string, receiverEmail string, subject string, body string) error {
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

	headers := []string{
		fmt.Sprintf("From: %s", fromHeader),
		fmt.Sprintf("To: %s", toHeader),
		fmt.Sprintf("Subject: %s", subject),
		fmt.Sprintf("Date: %s", date),
		fmt.Sprintf("Message-ID: %s", messageId),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}

	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

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

func (emailSender *smtpActionsEmailSender) SendSignupEmailAddressVerificationCode(emailAddress string, emailAddressVerificationCode string) error {
	subject := "Signup verification code"
	body := fmt.Sprintf("Your email address verification code is %s.", emailAddressVerificationCode)
	return emailSender.SendEmail("", emailAddress, subject, body)
}

func (emailSender *smtpActionsEmailSender) SendUserEmailAddressUpdateEmailVerificationCode(emailAddress string, displayName string, emailAddressVerificationCode string) error {
	subject := "Email update verification code"
	greeting := makeGreeting(displayName)
	codeMessage := fmt.Sprintf("You have made a request to update your email. Your verification code is %s.", emailAddressVerificationCode)
	body := fmt.Sprintf("%s\n\n%s", greeting, codeMessage)
	return emailSender.SendEmail(displayName, emailAddress, subject, body)
}

func (emailSender *smtpActionsEmailSender) SendUserPasswordResetTemporaryPassword(emailAddress string, displayName string, temporaryPassword string) error {
	subject := "Password reset temporary password"
	greeting := makeGreeting(displayName)
	passwordMessage := fmt.Sprintf("Your password reset temporary password is %s.", temporaryPassword)
	body := fmt.Sprintf("%s\n\n%s", greeting, passwordMessage)
	return emailSender.SendEmail(displayName, emailAddress, subject, body)
}

func (emailSender *smtpActionsEmailSender) SendUserSignedInNotification(emailAddress string, displayName string, time time.Time) error {
	subject := "Sign-in detected"
	greeting := makeGreeting(displayName)
	notificationMessage := fmt.Sprintf("We detected a sign-in to your account at %s (UTC).", time.UTC().Format("January 2, 2006 15:04:05"))
	body := fmt.Sprintf("%s\n\n%s", greeting, notificationMessage)
	return emailSender.SendEmail(displayName, emailAddress, subject, body)
}

func (emailSender *smtpActionsEmailSender) SendUserPasswordUpdatedNotification(emailAddress string, displayName string, time time.Time) error {
	subject := "Password updated"
	greeting := makeGreeting(displayName)
	notificationMessage := fmt.Sprintf("Your account password was updated at %s (UTC).", time.UTC().Format("January 2, 2006 15:04:05"))
	body := fmt.Sprintf("%s\n\n%s", greeting, notificationMessage)
	return emailSender.SendEmail(displayName, emailAddress, subject, body)
}

func (emailSender *smtpActionsEmailSender) SendUserEmailAddressUpdatedNotification(emailAddress string, displayName string, newEmailAddress string, time time.Time) error {
	subject := "Email updated"
	greeting := makeGreeting(displayName)
	notificationMessage := fmt.Sprintf("Your account email address was updated to %s at %s (UTC).", newEmailAddress, time.UTC().Format("January 2, 2006 15:04:05"))
	body := fmt.Sprintf("%s\n\n%s", greeting, notificationMessage)
	return emailSender.SendEmail(displayName, emailAddress, subject, body)
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
