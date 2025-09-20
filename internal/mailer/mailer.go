package mailer

import (
	"fmt"
	"log"
	"net/smtp"
)

type Mail struct {
	To      string
	Subject string
	Body    string
}

type Sender interface {
	Send(m Mail) error
}

type SMTPSender struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func (s *SMTPSender) Send(m Mail) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.User, s.Pass, s.Host)

	// Build message with proper headers
	msg := []byte("From: " + s.From + "\r\n" +
		"To: " + m.To + "\r\n" +
		"Subject: " + m.Subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
		"\r\n" +
		m.Body)

	err := smtp.SendMail(addr, auth, s.From, []string{m.To}, msg)
	if err != nil {
		return err
	}
	log.Printf("MAIL to=%s subject=%s body=%s", m.To, m.Subject, m.Body)
	return nil
}
