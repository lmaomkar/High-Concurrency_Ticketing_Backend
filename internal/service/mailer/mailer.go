package mailer

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/mailer"
)

type MailerService struct {
	log    *zap.Logger
	sender mailer.Sender
}

func NewMailerService(log *zap.Logger, sender mailer.Sender) *MailerService {
	return &MailerService{
		log:    log,
		sender: sender,
	}
}

func (m *MailerService) SendPaymentRequestEmail(userEmail string, eventName string, amount float64, paymentLink string) error {
	subject := fmt.Sprintf("Payment Required for %s", eventName)
	body := fmt.Sprintf(`
Dear User,

Your booking for "%s" is ready for payment.

Amount: $%.2f
Payment Link: %s

Please complete your payment within 15 minutes to secure your booking.

Best regards,
Evently Team
`, eventName, amount, paymentLink)

	mail := mailer.Mail{
		To:      userEmail,
		Subject: subject,
		Body:    body,
	}

	err := m.sender.Send(mail)
	if err != nil {
		m.log.Error("Failed to send payment request email", zap.Error(err), zap.String("email", userEmail))
		return err
	}

	m.log.Info("Payment request email sent", zap.String("email", userEmail), zap.String("event", eventName))
	return nil
}

func (m *MailerService) SendWaitlistPromotionEmail(userEmail string, eventName string) error {
	subject := fmt.Sprintf("Great News! You're off the waitlist for %s", eventName)
	body := fmt.Sprintf(`
Dear User,

Great news! A spot has opened up for "%s" and you're next in line!

You will receive a payment link soon.

Best regards,
Evently Team
`, eventName)

	mail := mailer.Mail{
		To:      userEmail,
		Subject: subject,
		Body:    body,
	}

	err := m.sender.Send(mail)
	if err != nil {
		m.log.Error("Failed to send waitlist promotion email", zap.Error(err), zap.String("email", userEmail))
		return err
	}

	m.log.Info("Waitlist promotion email sent", zap.String("email", userEmail), zap.String("event", eventName))
	return nil
}

func (m *MailerService) SendCancellationEmail(userEmail string, cancellationFee float64, paymentLink string) error {
	subject := "Booking Cancellation - Refund Information"
	body := fmt.Sprintf(`
Dear User,

Your booking has been cancelled.

Cancellation Fee: $%.2f
Refund Link: %s

Please use the refund link to process your refund.

Best regards,
Evently Team
`, cancellationFee, paymentLink)

	mail := mailer.Mail{
		To:      userEmail,
		Subject: subject,
		Body:    body,
	}

	err := m.sender.Send(mail)
	if err != nil {
		m.log.Error("Failed to send cancellation email", zap.Error(err), zap.String("email", userEmail))
		return err
	}

	m.log.Info("Cancellation email sent", zap.String("email", userEmail))
	return nil
}

func (m *MailerService) SendEventCancellationEmail(userEmail string, eventName string, refundAmount float64) error {
	subject := fmt.Sprintf("Event Cancelled: %s", eventName)
	body := fmt.Sprintf(`
Dear User,

We regret to inform you that the event "%s" has been cancelled.

Refund Amount: $%.2f

Your refund amount arrive shortly.

We apologize for any inconvenience.

Best regards,
Evently Team
`, eventName, refundAmount)

	mail := mailer.Mail{
		To:      userEmail,
		Subject: subject,
		Body:    body,
	}

	err := m.sender.Send(mail)
	if err != nil {
		m.log.Error("Failed to send event cancellation email", zap.Error(err), zap.String("email", userEmail))
		return err
	}

	m.log.Info("Event cancellation email sent", zap.String("email", userEmail), zap.String("event", eventName))
	return nil
}

func (m *MailerService) SendPasswordChangeOTPEmail(userEmail string, otp string) error {
	subject := "Password Change OTP"
	body := fmt.Sprintf(`
Dear User,

You have requested to change your password.

Your OTP is: %s

This OTP will expire in 15 minutes.

If you did not request this change, please ignore this email.

Best regards,
Evently Team
`, otp)

	mail := mailer.Mail{
		To:      userEmail,
		Subject: subject,
		Body:    body,
	}

	err := m.sender.Send(mail)
	if err != nil {
		m.log.Error("Failed to send password change OTP email", zap.Error(err), zap.String("email", userEmail))
		return err
	}

	m.log.Info("Password change OTP email sent", zap.String("email", userEmail))
	return nil
}
