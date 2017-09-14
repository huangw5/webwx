package wechat

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
)

const (
	// UserAgent is Chrome
	UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.79 Safari/537.36"
)

// GetUUID returns the UUID.
func GetUUID(client *http.Client) (string, error) {
	url := "https://login.weixin.qq.com/jslogin?appid=wx782c26e4c19acffb&fun=new&lang=us_EN"
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
	url := fmt.Sprintf("https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login?uuid=%s", uuid)
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

// Initialize logs on and returns basic info.
func Initialize(client *http.Client, url string) error {
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", "curl/7.35.0")
	req.Header.Add("Referer", "https://wx.qq.com/")
	//req.Header.Set("Content-Type", "text/plain;charset=utf-8")
	log.Printf("req: %+v", req)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP status: %s", resp.Status)
	}
	log.Printf("resp: %+v\n", resp)

	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
		//return fmt.Errorf("error reading body: %v", err)
	//}
	//log.Printf("body: %s", string(body))

	for _, c := range resp.Cookies() {
		log.Printf("cookie: %+v", c)
	}

	//type LoginInfo struct {
	//Ret         string `xml:"ret"`
	//Message     string `xml:"message"`
	//Skey        string `xml:"skey"`
	//Wxsid       string `xml:"wxsid"`
	//Wxuin       string `xml:"wxuin"`
	//PassTicket  string `xml:"pass_ticket"`
	//Isgrayscale int    `xml:"isgrayscale"`
	//}

	//li := &LoginInfo{}
	//if err := xml.Unmarshal(body, li); err != nil {
	//return fmt.Errorf("Failed to unmarshal body: %v", err)
	//}
	//log.Printf("Login Info: %+v", li)
	return nil
}
