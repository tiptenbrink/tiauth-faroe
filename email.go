package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"sync"
	"time"
)

type senderIdentity struct {
	// Can be empty string, in which case email is used
	name  string
	email string
}

type smtpActionsEmailSender struct {
	client   *smtp.Client
	identity senderIdentity
	m        sync.Mutex
}

// For standard username+password authentication, provide smtp.PlainAuth
func newSmtpEmailSenderWithAuth(identity senderIdentity, smtpServer string, smtpPort string, auth smtp.Auth) (*smtpActionsEmailSender, error) {
	serverAddr := smtpServer + ":" + smtpPort
	// We don't use SMTP dial because then the local name is set to "localhost", which can lead to
	// issues when using e.g. IP authentication
	conn, err := net.Dial("tcp4", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %v", serverAddr, err)
	}
	tlemailSenderonfig := &tls.Config{
		ServerName: smtpServer,
	}
	client, err := smtp.NewClient(conn, smtpServer)
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

	if err = client.StartTLS(tlemailSenderonfig); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to start TLS: %v", err)
	}

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to authenticate: %v", err)
		}
	}

	return &smtpActionsEmailSender{
		client:   client,
		identity: identity,
	}, nil
}

func newSmtpEmailSenderNoAuth(identity senderIdentity, smtpServer string, smtpPort string) (*smtpActionsEmailSender, error) {
	return newSmtpEmailSenderWithAuth(identity, smtpServer, smtpPort, nil)
}

// receiverName can be empty string, in which case the email is used.
func (emailSender *smtpActionsEmailSender) SendEmail(receiverName string, receiverEmail string, subject string, body string) error {
	var fromHeader, toHeader string

	if emailSender.identity.name != "" {
		fromHeader = fmt.Sprintf("%s <%s>", emailSender.identity.name, emailSender.identity.email)
	} else {
		fromHeader = emailSender.identity.email
	}

	if receiverName != "" {
		toHeader = fmt.Sprintf("%s <%s>", receiverName, receiverEmail)
	} else {
		toHeader = receiverEmail
	}

	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		fromHeader, toHeader, subject, body)

	emailSender.m.Lock()
	defer emailSender.m.Unlock()

	err := emailSender.client.Mail(emailSender.identity.email)
	if err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	if err = emailSender.client.Rcpt(receiverEmail); err != nil {
		return fmt.Errorf("failed to set recipient: %v", err)
	}

	writer, err := emailSender.client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %v", err)
	}

	_, err = writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close writer: %v", err)
	}

	return nil
}

func (emailSender *smtpActionsEmailSender) Close() error {
	emailSender.m.Lock()
	defer emailSender.m.Unlock()

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

func (emailSender *smtpActionsEmailSender) KeepAlive() error {
	emailSender.m.Lock()
	defer emailSender.m.Unlock()

	// NOOP command keeps connection alive
	if err := emailSender.client.Noop(); err != nil {
		return fmt.Errorf("keep-alive failed: %v", err)
	}

	return nil
}

func (emailSender *smtpActionsEmailSender) StartKeepAliveRoutine(interval time.Duration) chan bool {
	stopChan := make(chan bool)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := emailSender.KeepAlive(); err != nil {
					fmt.Printf("Keep-alive failed: %v\n", err)
				} else {
					fmt.Println("Keep-alive sent successfully")
				}
			case <-stopChan:
				fmt.Println("Keep-alive routine stopped")
				return
			}
		}
	}()

	return stopChan
}
