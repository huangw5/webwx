package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/huangw5/webwx/wechat"
)

func main() {
	fmt.Printf("Hello, World")

	c := &http.Client{
		Timeout: time.Second * 10,
	}
	wechat.GetUUID(c)
}
