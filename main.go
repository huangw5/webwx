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

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	c := wechat.NewClient()
	w := &wechat.Wechat{Client: c}
	if err := w.Login(); err != nil {
		glog.Exitf("Failed to login: %v", err)
	}
	//ticker := time.NewTicker(syncInterval)
	//for _ = range ticker.C {
		sr, err := w.SyncCheck()
		if err != nil || sr.Retcode != "0" {
			glog.Exitf("SyncCheck failed -- Res: %+v, err: %v", sr, err)
		}
		if sr.Selector == "6" {
			glog.Infof("Got new WeChat messages")
		}
	//}
}
