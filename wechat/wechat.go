package wechat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
)

const (
	// UserAgent is Chrome.
	UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.79 Safari/537.36"
)

// NowUnixMilli returns UTC time of milliseconds since.
func NowUnixMilli() int {
	return int(time.Now().UnixNano() / 1000000)
}

// LoginInfo contains the login information.
type LoginInfo struct {
	Ret         string `xml:"ret"`
	Message     string `xml:"message"`
	Skey        string `xml:"skey"`
	Wxsid       string `xml:"wxsid"`
	Wxuin       string `xml:"wxuin"`
	PassTicket  string `xml:"pass_ticket"`
	Isgrayscale int    `xml:"isgrayscale"`
}

// BaseRequest is the base Request to the server.
type BaseRequest struct {
	DeviceID string `json:"DeviceID"`
	Sid      string `json:"Sid"`
	Skey     string `json:"Skey"`
	Uin      string `json:"Uin"`
}

// BaseJSON is.
type BaseJSON struct {
	BaseRequest *BaseRequest `json:"BaseRequest"`
}


// GetUUID returns the UUID.
func GetUUID(client *http.Client) (string, error) {
	url := fmt.Sprintf("https://login.weixin.qq.com/jslogin?appid=%s&fun=new&lang=us_EN&_=%d", "wx782c26e4c19acffb", NowUnixMilli())
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP status: %s", resp.Status)
	}
	log.Printf("resp: %+v\n", resp)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile("window.QRLogin.uuid = \"([^\"]+)\"")
	matches := re.FindStringSubmatch(string(body))
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid body: %s", body)
	}
	return matches[1], nil
}

// GetQRCode retrieves and saves the QR image to a file.
func GetQRCode(client *http.Client, uuid string, f *os.File) error {
	url := fmt.Sprintf("https://login.weixin.qq.com/qrcode/%s?t=webwx", uuid)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP status: %s", resp.Status)
	}
	log.Printf("resp: %+v\n", resp)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading body: %v", err)
	}
	if _, err := f.Write(body); err != nil {
		return fmt.Errorf("error writing QR image: %v", err)
	}
	log.Printf("Successfully save QR image to %s", f.Name())
	return nil
}

// WaitUntilLoggedIn waits until user clicks login or timed out. Returns a redirect_uri.
func WaitUntilLoggedIn(client *http.Client, uuid string) (string, error) {
	url := fmt.Sprintf("https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login?uuid=%s&_=%d", uuid, NowUnixMilli())
	re := regexp.MustCompile("window.redirect_uri=\"([^\"]+)\"")
	const tries = 10
	for i := 0; i < tries; i++ {
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("Error on GET: %v", err)
			continue
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			continue
		}
		log.Printf("Body: %s", body)
		matches := re.FindStringSubmatch(string(body))
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", errors.New("timeout when waiting for user to login")
}

// Login logs on and returns basic info.
func Login(client *http.Client, url string) (*LoginInfo, error) {
	// First access the redirect_uri.
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", UserAgent)
	req.Header.Add("Referer", "https://wx.qq.com/")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()
	log.Printf("resp: %+v\n", resp)

	// Expect 301.
	if resp.StatusCode != 301 {
		return nil, fmt.Errorf("HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}

	li := &LoginInfo{}
	if err := xml.Unmarshal(body, li); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal body: %v", err)
	}
	log.Printf("Login Info: %+v", li)

	// Then init.
	bj := &BaseJSON{
		BaseRequest: &BaseRequest{
			Uin:      li.Wxuin,
			Sid:      li.Wxsid,
			Skey:     li.Skey,
			DeviceID: fmt.Sprintf("e%d", time.Now().Unix()),
		},
	}
	b, err := json.Marshal(bj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %v", err)
	}
	url2 := fmt.Sprintf("https://web.wechat.com/cgi-bin/mmwebwx-bin/webwxinit?pass_ticket=%s&skey=%s&r=%d", li.PassTicket, li.Skey, NowUnixMilli())
	req2, _ := http.NewRequest("POST", url2, bytes.NewBuffer(b))
	req2.Header.Add("User-Agent", UserAgent)
	req2.Header.Add("Referer", "https://wx.qq.com/")
	resp2, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("error on POST: %v", err)
	}
	defer resp2.Body.Close()
	log.Printf("resp: %+v\n", resp2)

	body2, err := ioutil.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	log.Printf("body: %s", string(body2))

	return li, nil
}
