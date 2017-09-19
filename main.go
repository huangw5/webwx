package main

import (
	"flag"
	"time"

	"github.com/golang/glog"
	"github.com/huangw5/webwx/wechat"
)

const (
	syncInterval = 28 * time.Second
)

var (
	appid = flag.String("appid", "wx782c26e4c19acffb", "App ID")
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

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
			glog.Exitf("WebwxSync failed: %v", err)
		}
		var newMessages []string
		for _, msg := range ws.AddMsgList {
			if msg.MsgType != 1 {
				// Skip non-text messages.
				continue
			}
			if _, ok := messages[msg.MsgID]; !ok {
				newMessages = append(newMessages, msg.Content)
				messages[msg.MsgID] = true
				glog.Infof("Message: %s", msg.Content)
			}
		}
	}
}
