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
	syncInterval   = 3 * time.Second
	notifyInterval = time.Minute
)

var (
	appid    = flag.String("appid", "wx782c26e4c19acffb", "App ID")
	from     = flag.String("from", "", "Email sender")
	to       = flag.String("to", "", "Email recipient")
	password = flag.String("password", "", "Email password")
	smtpAddr = flag.String("smtp", "smtp.gmail.com:587", "SMTP Address")
	detail   = flag.Bool("detail", true, "Wether or not show detailed messages in emails")
	forward  = flag.String("forward", "", "The nickname to which the messages are forwarded")
)

func sendEmail(m *mail.Mail, newChan chan *wechat.AddMsg) error {
	var l []string
	for i := 0; i < len(newChan); i++ {
		msg := <-newChan
		l = append(l, fmt.Sprintf("%s: %s", msg.NickName, msg.Content))
	}
	body := strings.Join(l, "\n")
	if !*detail {
		body = ""
	}
	if err := m.Send([]string{*to}, "New WeChat messages", body); err != nil {
		return err
	}
	glog.Infof("Successfully sent to %s", *to)
	return nil
}

func forwardMsg(w *wechat.Wechat, toUserName string, newChan chan *wechat.AddMsg) error {
	var l []string
	for i := 0; i < len(newChan); i++ {
		msg := <-newChan
		l = append(l, fmt.Sprintf("%s: %s", msg.NickName, msg.Content))
	}
	body := strings.Join(l, "\n")
	toSend := &wechat.Msg{
		Content:    body,
		ToUserName: toUserName,
		Type:       1,
	}
	return w.SendMsg(toSend)
}

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
	if *forward != "" {
		glog.Infof("New messages will be forwarded to %s", *forward)
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
	newChan := make(chan *wechat.AddMsg, 9999)
	syncChan := time.NewTicker(syncInterval).C
	notifyChan := time.NewTicker(notifyInterval).C
	for ; true; <-syncChan {
		sr, err := w.SyncCheck()
		if err != nil || sr.Retcode != "0" {
			if m != nil {
				m.Send([]string{*to}, fmt.Sprintf("SyncCheck failed -- Res: %+v, err: %v", sr, err), "")
			}
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
				// Skip non-displayable messages.
				continue
			}
			if _, ok := allMessages[msg.MsgID]; !ok {
				allMessages[msg.MsgID] = true
				glog.Info(fmt.Sprintf("%s: %s", msg.NickName, msg.Content))
				// Do not notify group chat messages.
				if !strings.HasPrefix(msg.FromUserName, "@@") {
					newChan <- msg
				}
			}
		}
		select {
		case <-notifyChan:
			if len(newChan) > 0 {
				if m != nil {
					if err := sendEmail(m, newChan); err != nil {
						glog.Warningf("Failed to send email: %v", err)
					}
				}
				if user, ok := w.Contacts[*forward]; *forward != "" && ok {
					if err := forwardMsg(w, user.UserName, newChan); err != nil {
						glog.Warningf("Failed to forward: %v")
					}
				} else {
					glog.Warningf("Unable to forward to: %s", user)
				}
			}
		default:
			glog.V(1).Infof("Not sending email")
		}
	}
}
