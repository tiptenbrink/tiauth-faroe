package tiauth

import (
	"time"
)

// backendEmailSender implements faroe's EmailSenderInterface by delegating to the Python backend.
// All email sending, token storage, and template rendering is handled by Python.
type backendEmailSender struct {
	backendClient *BackendClient
}

func (s *backendEmailSender) SendSignupEmailAddressVerificationCode(email string, code string) error {
	return s.backendClient.SendEmail(EmailRequest{
		Type:  "signup_verification",
		Email: email,
		Code:  code,
	})
}

func (s *backendEmailSender) SendUserEmailAddressUpdateEmailVerificationCode(email string, displayName string, code string) error {
	return s.backendClient.SendEmail(EmailRequest{
		Type:        "email_update_verification",
		Email:       email,
		DisplayName: displayName,
		Code:        code,
	})
}

func (s *backendEmailSender) SendUserPasswordResetTemporaryPassword(email string, displayName string, tempPassword string) error {
	return s.backendClient.SendEmail(EmailRequest{
		Type:        "password_reset",
		Email:       email,
		DisplayName: displayName,
		Code:        tempPassword,
	})
}

func (s *backendEmailSender) SendUserSignedInNotification(email string, displayName string, t time.Time) error {
	return s.backendClient.SendEmail(EmailRequest{
		Type:        "signin_notification",
		Email:       email,
		DisplayName: displayName,
		Timestamp:   t.UTC().Format("January 2, 2006 15:04:05"),
	})
}

func (s *backendEmailSender) SendUserPasswordUpdatedNotification(email string, displayName string, t time.Time) error {
	return s.backendClient.SendEmail(EmailRequest{
		Type:        "password_updated",
		Email:       email,
		DisplayName: displayName,
		Timestamp:   t.UTC().Format("January 2, 2006 15:04:05"),
	})
}

func (s *backendEmailSender) SendUserEmailAddressUpdatedNotification(email string, displayName string, newEmail string, t time.Time) error {
	return s.backendClient.SendEmail(EmailRequest{
		Type:        "email_updated",
		Email:       email,
		DisplayName: displayName,
		NewEmail:    newEmail,
		Timestamp:   t.UTC().Format("January 2, 2006 15:04:05"),
	})
}
