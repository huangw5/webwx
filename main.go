package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/huangw5/webwx/mail"
	"github.com/huangw5/webwx/wechat"
)

const (
	syncInterval  = 27 * time.Second
	emailInterval = time.Minute
)

var (
	appid    = flag.String("appid", "wx782c26e4c19acffb", "App ID")
	from     = flag.String("from", "", "Email sender")
	to       = flag.String("to", "", "Email recipient")
	password = flag.String("password", "", "Email password")
	smtpAddr = flag.String("smtp", "smtp.gmail.com:587", "SMTP Address")
	detail   = flag.Bool("detail", true, "Wether or not show detailed messages in emails")
)

func main() {
	flag.Parse()
	flag.Lookup("alsologtostderr").Value.Set("true")

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

	allMessages := make(map[string]bool)
	newChan := make(chan string, 10000)
	syncChan := time.NewTicker(syncInterval).C
	emailChan := time.NewTicker(emailInterval).C
	for ; true; <-syncChan {
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
		for _, msg := range ws.AddMsgList {
			if msg.MsgType != 1 && msg.MsgType != 3 && msg.MsgType != 34 && msg.MsgType != 43 && msg.MsgType != 62 && msg.MsgType != 47 {
				// Skip non-text messages.
				continue
			}
			if _, ok := allMessages[msg.MsgID]; !ok {
				text := fmt.Sprintf("%s: %s", msg.NickName, msg.Content)
				newChan <- text
				allMessages[msg.MsgID] = true
				glog.Info(text)
			}
		}
		select {
		case <-emailChan:
			if len(newChan) > 0 && m != nil {
				var l []string
				for i := 0; i < len(newChan); i++ {
					l = append(l, <-newChan)
				}
				body := strings.Join(l, "\n")
				if !*detail {
					body = ""
				}
				sub := fmt.Sprintf("New WeChat messages (%d)", len(l))
				if err := m.Send([]string{*to}, sub, body); err != nil {
					glog.Warningf("Send email failed: %v", err)
				} else {
					glog.Infof("Sent successfully")
				}
			}
		default:
			glog.V(1).Infof("Not sending email")
		}
	}
}
