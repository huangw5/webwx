package wechat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/publicsuffix"
)

const (
	// userAgent is Chrome.
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.79 Safari/537.36"
	loginHost = "https://login.weixin.qq.com"
	webHost   = "https://web.wechat.com"
)

var (
	syncHosts = []string{"https://webpush2.wechat.com", "https://webpush.wechat.com"}
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

// SyncKey is sync key.
type SyncKey struct {
	Count int              `json:"Count"`
	List  []map[string]int `json:"List"`
}

func (s SyncKey) String() string {
	var entries []string
	for _, m := range s.List {
		entries = append(entries, fmt.Sprintf("%d_%d", m["Key"], m["Val"]))
	}
	return strings.Join(entries, "|")
}

// BaseRequestJSON is.
type BaseRequestJSON struct {
	BaseRequest *BaseRequest `json:"BaseRequest"`
	SyncKey     *SyncKey     `json:"SyncKey"`
	RR          int          `json:"rr"`
}

// BaseResponse is.
type BaseResponse struct {
	Ret    int    `json:"Ret"`
	ErrMsg string `json:"ErrMsg"`
}

// AddMsg is new message.
type AddMsg struct {
	Content string `json:"Content"`
}

// BaseResponseJSON is.
type BaseResponseJSON struct {
	BaseResponse *BaseResponse `json:"BaseResponse"`
	AddMsgCount  int           `json:"AddMsgCount"`
	AddMsgList   []*AddMsg      `json:"AddMsgList"`
}

// SyncRes holds the result for syncing with the server.
//    retcode: 0    successful
//	     1100 logout
//	     1101 login otherwhere
//    selector: 0 nothing
//	      2 message
//	      6 unkonwn
//	      7 webwxsync
type SyncRes struct {
	Retcode  string
	Selector string
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
			Timeout: time.Second * 30,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
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
}

// GetUUID returns the UUID.
func (w *Wechat) GetUUID() (string, error) {
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

// GetQRCode retrieves and saves the QR image to a file.
func (w *Wechat) GetQRCode(uuid string, f *os.File) error {
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

// WaitUntilLoggedIn waits until user clicks login or timed out. Returns a redirect_uri.
func (w *Wechat) WaitUntilLoggedIn(uuid string) (string, error) {
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

// Init logs on and returns basic info.
func (w *Wechat) Init(url string) (*BaseRequestJSON, error) {
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
		webHost, li.PassTicket, li.Skey, NowUnixMilli())
	resp2, err := w.Client.Do("POST", url2, bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("error on POST: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := ioutil.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	if err := json.Unmarshal(body2, bj); err != nil {
		return nil, fmt.Errorf("error on unmarshal: %v", err)
	}
	return bj, nil
}

// Login logs onto the server.
func (w *Wechat) Login() error {
	glog.Infof("Getting UUID...")
	uuid, err := w.GetUUID()
	if err != nil {
		return fmt.Errorf("error on GetUUID(): %v", err)
	}
	glog.Infof("UUID: %s", uuid)

	glog.Infof("Getting QR code...")
	f, err := os.Create("QR.jpg")
	if err != nil {
		return fmt.Errorf("error on createing QR file: %v", err)
	}
	if err := w.GetQRCode(uuid, f); err != nil {
		return fmt.Errorf("error on getting QR code: %v", err)
	}

	glog.Infof("Please scan QR code from you phone: %s", f.Name())
	rurl, err := w.WaitUntilLoggedIn(uuid)
	if err != nil {
		return fmt.Errorf("error on scanning the QR code")
	}
	glog.Infof("Got init URL: %s", rurl)

	glog.Infof("Initializing wechat...")
	w.BaseRequestJSON, err = w.Init(rurl)
	if err != nil {
		return fmt.Errorf("error on init: %v", err)
	}
	glog.Infof("Got BaseRequestJSON: %+v", w.BaseRequestJSON)
	glog.Infof("Login successfully")
	return nil
}

// SyncCheck synchronizes with the server.
func (w *Wechat) SyncCheck() (*SyncRes, error) {
	glog.Infof("SyncCheck...")
	syncRes := &SyncRes{}
	var err error
	for _, host := range syncHosts {
		syncRes, err = w.syncCheckHelper(host)
		if err == nil && syncRes.Retcode == "0" {
			glog.Infof("Successfully synccheck: %+v", syncRes)
			return syncRes, nil
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
	w.BaseRequestJSON.RR = NowUnixMilli()
	url := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/webwxsync?sid=%s&skey=%s&r=%d", webHost, w.BaseRequestJSON.BaseRequest.Sid, w.BaseRequestJSON.BaseRequest.Skey, w.BaseRequestJSON.RR)
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
	glog.V(1).Infof("body: %s", string(body))
	br := &BaseResponseJSON{}
	if err := json.Unmarshal(body, br); err != nil {
		return nil, fmt.Errorf("error on unmarshal: %v", err)
	}
	return br, nil
}
