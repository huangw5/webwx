package mail

import (
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// Mail is for sending emails.
// Example:
//	m := &mail.Mail{
//		From:     "xxx@gmail.com",
//		Pass:     "xxx",
//		SMTPAddr: "smtp.gmail.com:587",
//	}
//	if err := m.Send([]string{"yyy@gmail.com"}, "Hello", "This is a message"); err != nil {
//		glog.Exitf("Send failed: %v", err)
//	}
type Mail struct {
	From     string
	Pass     string
	SMTPAddr string
}

// Send sends a message.
func (m *Mail) Send(to []string, subject string, body string) error {
	host, _, err := net.SplitHostPort(m.SMTPAddr)
	if err != nil {
		return err
	}
	auth := smtp.PlainAuth("", m.From, m.Pass, host)
	msg := []byte(fmt.Sprintf("To: %s\r\n"+"Subject: %s\r\n"+"\r\n"+"%s\r\n", strings.Join(to, ","), subject, body))
	return smtp.SendMail(m.SMTPAddr, auth, m.From, to, msg)
}
