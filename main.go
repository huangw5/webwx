package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/huangw5/webwx/wechat"
)

func main() {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")

	c := wechat.NewClient()
	w := &wechat.Wechat{Client: c}
	bj, err := w.Login()
	if err != nil {
		glog.Exitf("Failed to login: %v", err)
	}
	glog.Infof("Got BaseJSON: %+v", bj)
}
