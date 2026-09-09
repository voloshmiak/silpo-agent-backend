package email

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
)

type Mailer struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func NewMailer(host, port, username, password, from string) *Mailer {
	if from == "" {
		from = username
	}
	return &Mailer{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
	}
}

// SendPassword sends a welcoming email with the generated account password.
func (m *Mailer) SendPassword(toEmail, password string) error {
	if m.Host == "" || m.Username == "" {
		log.Printf("[MAILER MOCK] SMTP not configured. Generated password for %s is: %s", toEmail, password)
		return nil
	}

	subject := "Ваш доступ до Silpo Fit AI"
	body := fmt.Sprintf(
		"Вітаємо!\n\n"+
			"Ви успішно пройшли онбординг у Silpo Fit AI.\n"+
			"Ваші дані для наступного входу в акаунт:\n\n"+
			"Логін (Email): %s\n"+
			"Пароль: %s\n\n"+
			"Ви можете змінити цей пароль у налаштуваннях профілю.\n\n"+
			"З повагою,\nКоманда Silpo Fit AI",
		toEmail, password,
	)

	msg := "From: " + m.From + "\r\n" +
		"To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body

	addr := net.JoinHostPort(m.Host, m.Port)
	auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)

	// Try standard SendMail
	err := smtp.SendMail(addr, auth, m.From, []string{toEmail}, []byte(msg))
	if err != nil {
		// Fallback for TLS / port 465 if needed
		if strings.Contains(err.Error(), "unencrypted connection") || m.Port == "465" {
			tlsconfig := &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         m.Host,
			}
			conn, dialErr := tls.Dial("tcp", addr, tlsconfig)
			if dialErr != nil {
				log.Printf("[ERROR] SMTP TLS dial failed: %v", dialErr)
				return dialErr
			}
			c, clientErr := smtp.NewClient(conn, m.Host)
			if clientErr != nil {
				log.Printf("[ERROR] SMTP client creation failed: %v", clientErr)
				return clientErr
			}
			defer c.Close()
			if err = c.Auth(auth); err != nil {
				return err
			}
			if err = c.Mail(m.From); err != nil {
				return err
			}
			if err = c.Rcpt(toEmail); err != nil {
				return err
			}
			w, dataErr := c.Data()
			if dataErr != nil {
				return dataErr
			}
			_, _ = w.Write([]byte(msg))
			_ = w.Close()
			return c.Quit()
		}
		log.Printf("[ERROR] failed to send email to %s: %v", toEmail, err)
		return err
	}

	log.Printf("[INFO] credentials email sent successfully to %s", toEmail)
	return nil
}
