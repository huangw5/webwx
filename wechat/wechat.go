package wechat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/publicsuffix"
)

const (
	// userAgent is Chrome.
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.79 Safari/537.36"
	loginHost = "https://login.weixin.qq.com"
)

var (
	syncHosts = map[string][]string{
		"web.wechat.com": []string{
			"https://webpush.web.wechat.com",
			"https://webpush2.wechat.com",
			"https://webpush.wechat.com",
		},
	}
	webHosts = map[string]string{
		"web.wechat.com": "https://web.wechat.com",
	}
)

// NowUnixMilli returns UTC time of milliseconds since.
func NowUnixMilli() int {
	return int(time.Now().UnixNano() / 1000000)
}

// HTTPClient wraps http.Client.
type HTTPClient interface {
	Do(method, url string, body io.Reader) (*http.Response, error)
}

type httpClient struct {
	c *http.Client
}

func (hc *httpClient) Do(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Referer", "https://wx.qq.com/")
	glog.V(1).Infof("Request: %s %s\n", req.Method, req.URL)
	resp, err := hc.c.Do(req)
	glog.V(1).Infof("Response: %+v\n", resp)
	return resp, err
}

// NewClient creates a new instance of httpClient.
func NewClient() HTTPClient {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		glog.Fatal(err)
	}
	return &httpClient{
		&http.Client{
			Timeout: time.Second * 50,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   30 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}
}

// Wechat is an instance of wechat ID.
type Wechat struct {
	Client          HTTPClient
	BaseRequestJSON *BaseRequestJSON
	LoginInfo       *LoginInfo
	AppID           string
	contacts        map[string]string
	host            string
}

// getUUID returns the UUID.
func (w *Wechat) getUUID() (string, error) {
	url := fmt.Sprintf("%s/jslogin?appid=%s&fun=new&lang=us_EN&_=%d",
		loginHost, w.AppID, NowUnixMilli())
	resp, err := w.Client.Do("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP status: %s", resp.Status)
	}

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

// getQRCode retrieves and saves the QR image to a file.
func (w *Wechat) getQRCode(uuid string, f *os.File) error {
	url := fmt.Sprintf("%s/qrcode/%s?t=webwx", loginHost, uuid)
	resp, err := w.Client.Do("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading body: %v", err)
	}
	if _, err := f.Write(body); err != nil {
		return fmt.Errorf("error writing QR image: %v", err)
	}
	glog.Infof("Successfully save QR image to %s", f.Name())
	return nil
}

// waitUntilLoggedIn waits until user clicks login or timed out. Returns a redirect_uri.
func (w *Wechat) waitUntilLoggedIn(uuid string) (string, error) {
	url := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/login?uuid=%s&_=%d",
		loginHost, uuid, NowUnixMilli())
	re := regexp.MustCompile("window.redirect_uri=\"([^\"]+)\"")
	const tries = 10
	for i := 0; i < tries; i++ {
		resp, err := w.Client.Do("GET", url, nil)
		if err != nil {
			glog.Infof("Error on GET: %v", err)
			continue
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Infof("Error reading body: %v", err)
			continue
		}
		glog.V(1).Infof("Body: %s", body)
		matches := re.FindStringSubmatch(string(body))
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", errors.New("timeout when waiting for user to login")
}

// init logs on and returns basic info.
func (w *Wechat) init(url string) (*BaseRequestJSON, error) {
	// First access the redirect_uri.
	resp, err := w.Client.Do("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()

	// Expect 301.
	if resp.StatusCode != 301 {
		return nil, fmt.Errorf("HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	glog.V(1).Infof("body: %s", string(body))

	li := &LoginInfo{}
	if err := xml.Unmarshal(body, li); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal body: %v", err)
	}

	// Then init.
	bj := &BaseRequestJSON{
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
	w.LoginInfo = li
	url2 := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/webwxinit?pass_ticket=%s&skey=%s&r=%d",
		webHosts[w.host], li.PassTicket, li.Skey, NowUnixMilli())
	resp2, err := w.Client.Do("POST", url2, bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("error on POST: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := ioutil.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	glog.V(1).Infof("webwxinit: %s", string(body2))
	if err := json.Unmarshal(body2, bj); err != nil {
		return nil, fmt.Errorf("error on unmarshal: %v", err)
	}
	return bj, nil
}

// Login logs onto the server.
func (w *Wechat) Login() error {
	glog.Infof("Getting UUID...")
	uuid, err := w.getUUID()
	if err != nil {
		return fmt.Errorf("error on getUUID(): %v", err)
	}
	glog.Infof("UUID: %s", uuid)

	glog.Infof("Getting QR code...")
	f, err := os.Create("QR.jpg")
	if err != nil {
		return fmt.Errorf("error on createing QR file: %v", err)
	}
	if err := w.getQRCode(uuid, f); err != nil {
		return fmt.Errorf("error on getting QR code: %v", err)
	}

	pwd, _ := os.Getwd()
	glog.Infof("Please scan QR code from you phone:\nfile://%s", path.Join(pwd, f.Name()))
	rurl, err := w.waitUntilLoggedIn(uuid)
	if err != nil {
		return fmt.Errorf("error on scanning the QR code")
	}
	glog.Infof("Got init URL: %s", rurl)
	u, err := url.Parse(rurl)
	if err != nil {
		return fmt.Errorf("error on parsing url: %v", err)
	}
	w.host = u.Hostname()
	glog.Infof("Updated host to %s", w.host)

	glog.Infof("Initializing wechat...")
	w.BaseRequestJSON, err = w.init(rurl)
	if err != nil {
		return fmt.Errorf("error on init: %v", err)
	}
	glog.Infof("Got BaseRequestJSON: %+v", w.BaseRequestJSON)
	glog.Infof("Login successfully")

	glog.Infof("Getting contacts...")
	w.contacts, err = w.GetContacts()
	if err != nil {
		glog.Warningf("Failed to get contacts: %v", err)
	}
	if u := w.BaseRequestJSON.User; u != nil {
		w.contacts[u.UserName] = u.NickName
	}
	glog.Infof("Got %d contacts", len(w.contacts))
	return nil
}

// GetContacts retrieves contacts.
func (w *Wechat) GetContacts() (map[string]string, error) {
	url := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/webwxgetcontact?r=%d", webHosts[w.host], NowUnixMilli())
	resp, err := w.Client.Do("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error on POST: %v", err)
	}
	defer resp.Body.Close()

	// Expect 301.
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	br := &BaseResponseJSON{}
	if err := json.Unmarshal(body, br); err != nil {
		return nil, fmt.Errorf("error on unmarshal: %v", err)
	}
	contacts := make(map[string]string)
	for _, member := range br.MemberList {
		contacts[member.UserName] = member.NickName
	}
	return contacts, nil
}

// SyncCheck synchronizes with the server.
func (w *Wechat) SyncCheck() (*SyncRes, error) {
	syncRes := &SyncRes{}
	var err error
	for i := 0; i < 3; i++ {
		for _, host := range syncHosts[w.host] {
			glog.Infof("SyncCheck on %s. Attempt: %d", host, i+1)
			syncRes, err = w.syncCheckHelper(host)
			if err == nil && syncRes.Retcode == "0" {
				glog.Infof("Successfully synccheck: %+v", syncRes)
				return syncRes, nil
			}
			time.Sleep(time.Second)
		}
	}
	return syncRes, err
}

func (w *Wechat) syncCheckHelper(host string) (*SyncRes, error) {
	br := w.BaseRequestJSON.BaseRequest
	url := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/synccheck?r=%d&sid=%s&uin=%s&skey=%s&deviceid=%s&synckey=%s&_=%d",
		host, NowUnixMilli(), br.Sid, br.Uin, br.Skey, br.DeviceID, w.BaseRequestJSON.SyncKey.String(), NowUnixMilli())

	resp, err := w.Client.Do("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error on GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	re := regexp.MustCompile("window.synccheck={retcode:\"([^\"]+)\",selector:\"([^\"]+)\"}")
	matches := re.FindStringSubmatch(string(body))
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid response: %s", string(body))
	}
	return &SyncRes{Retcode: matches[1], Selector: matches[2]}, nil
}

// WebwxSync retrieves new messages.
func (w *Wechat) WebwxSync() (*BaseResponseJSON, error) {
	var br *BaseResponseJSON
	var err error
	for i := 0; i < 3; i++ {
		host := webHosts[w.host]
		glog.Infof("WebwxSync on %s. Attemp: %d", host, i+0)
		br, err := w.webwxsyncHelper(host)
		if err == nil {
			glog.Infof("Successfully WebwxSync: %+v", br.BaseResponse)
			return br, err
		}
		time.Sleep(time.Second)
	}
	return br, err
}

func (w *Wechat) webwxsyncHelper(host string) (*BaseResponseJSON, error) {
	w.BaseRequestJSON.RR = NowUnixMilli()
	url := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/webwxsync?sid=%s&skey=%s&r=%d", host, w.BaseRequestJSON.BaseRequest.Sid, w.BaseRequestJSON.BaseRequest.Skey, w.BaseRequestJSON.RR)
	b, err := json.Marshal(w.BaseRequestJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %v", err)
	}
	resp, err := w.Client.Do("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("error on POST: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	glog.V(1).Infof("WebwxSync: %s", string(body))
	br := &BaseResponseJSON{}
	if err := json.Unmarshal(body, br); err != nil {
		return nil, fmt.Errorf("error on unmarshal: %v", err)
	}
	for _, msg := range br.AddMsgList {
		if n, ok := w.contacts[msg.FromUserName]; ok {
			msg.NickName = n
		} else {
			msg.NickName = msg.FromUserName
		}
	}
	return br, nil
}
