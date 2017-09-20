package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/huangw5/webwx/wechat"
	"github.com/huangw5/webwx/mail"
)

const (
	syncInterval = 28 * time.Second
)

var (
	appid    = flag.String("appid", "wx782c26e4c19acffb", "App ID")
	from     = flag.String("from", "", "Email sender")
	to       = flag.String("to", "", "Email recipient")
	password = flag.String("password", "", "Email password")
	smtpAddr = flag.String("smtp", "smtp.gmail.com:587", "SMTP Address")
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	var m *mail.Mail
	if *from != "" && *to != "" && *password != "" {
		m = &mail.Mail{
			From:     *from,
			Pass:     *password,
			SMTPAddr: *smtpAddr,
		}
		glog.Infof("New messages will be sent to %s", *to)
	}

	c := wechat.NewClient()
	w := &wechat.Wechat{
		Client: c,
		AppID:  *appid,
	}
	if err := w.Login(); err != nil {
		glog.Exitf("Failed to login: %v", err)
	}
	messages := make(map[string]bool)
	ticker := time.NewTicker(syncInterval)
	for ; true; <-ticker.C {
		sr, err := w.SyncCheck()
		if err != nil || sr.Retcode != "0" {
			glog.Exitf("SyncCheck failed -- Res: %+v, err: %v", sr, err)
		}
		if sr.Selector == "0" {
			continue
		}
		ws, err := w.WebwxSync()
		if err != nil {
			glog.Errorf("WebwxSync failed: %v", err)
			continue
		}
		var newMessages []string
		for _, msg := range ws.AddMsgList {
			if msg.MsgType != 1 {
				// Skip non-text messages.
				continue
			}
			if _, ok := messages[msg.MsgID]; !ok {
				text := fmt.Sprintf("%s: %s", msg.NickName, msg.Content)
				newMessages = append(newMessages, text)
				messages[msg.MsgID] = true
				glog.Info(text)
			}
		}
		if len(newMessages) > 0 && m != nil {
			sub := fmt.Sprintf("You got %d new WeChat messages", len(newMessages))
			body := strings.Join(newMessages, "\n")
			if err := m.Send([]string{*to}, sub, body); err != nil {
				glog.Warningf("Send email failed: %v", err)
			} else {
				glog.Infof("Sent successfully")
			}
		}
	}
}
