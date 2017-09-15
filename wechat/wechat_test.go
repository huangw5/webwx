package wechat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"golang.org/x/net/publicsuffix"
)

func newClient() HTTPClient {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		log.Fatal(err)
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

func TestGetUUID(t *testing.T) {
	w := &Wechat{Client: newClient()}
	uuid, err := w.GetUUID()
	if err != nil {
		t.Fatalf("GetUUID failed: %v", err)
	}
	if uuid == "" {
		t.Errorf("Invalid uuid: %s", uuid)
	}
	log.Printf("uuid: %s", uuid)
}

func TestGetQRCode(t *testing.T) {
	w := &Wechat{Client: newClient()}
	uuid, err := w.GetUUID()
	if err != nil {
		t.Fatalf("GetUUID failed: %v", err)
	}
	log.Printf("uuid: %s", uuid)
	tmp, err := ioutil.TempFile("", "QR")
	if err != nil {
		t.Fatalf("TempFile failed: %v", err)
	}
	defer tmp.Close()
	if err := w.GetQRCode(uuid, tmp); err != nil {
		t.Errorf("GetQRCode failed; %v", err)
	}
}

func TestWaitUntilLoggedIn(t *testing.T) {
	w := &Wechat{Client: newClient()}
	uuid, err := w.GetUUID()
	if err != nil {
		t.Fatalf("GetUUID failed: %v", err)
	}
	log.Printf("uuid: %s", uuid)
	tmp, err := ioutil.TempFile("", "QR")
	if err != nil {
		t.Fatalf("TempFile failed: %v", err)
	}
	defer tmp.Close()
	defer os.Remove(tmp.Name())
	if err := w.GetQRCode(uuid, tmp); err != nil {
		t.Fatalf("GetQRCode failed; %v", err)
	}
	log.Printf("Please scan %s from your phone\n", tmp.Name())
	uri, err := w.WaitUntilLoggedIn(uuid)
	if err != nil {
		t.Fatalf("WaitUntilLoggedIn failed: %v", err)
	}
	log.Printf("uri: %s", uri)
}

func TestLogin(t *testing.T) {
	w := &Wechat{Client: newClient()}
	uuid, err := w.GetUUID()
	if err != nil {
		t.Fatalf("GetUUID failed: %v", err)
	}
	log.Printf("uuid: %s", uuid)
	tmp, err := ioutil.TempFile("", "QR")
	if err != nil {
		t.Fatalf("TempFile failed: %v", err)
	}
	defer tmp.Close()
	defer os.Remove(tmp.Name())
	if err := w.GetQRCode(uuid, tmp); err != nil {
		t.Fatalf("GetQRCode failed; %v", err)
	}
	log.Printf("Please scan %s from your phone\n", tmp.Name())
	uri, err := w.WaitUntilLoggedIn(uuid)
	if err != nil {
		t.Fatalf("WaitUntilLoggedIn failed: %v", err)
	}
	log.Printf("uri: %s", uri)

	if _, err := w.Login(uri); err != nil {
		t.Fatalf("Login failed: %v", err)
	}
}

func TestMarshal(t *testing.T) {
	bj := &BaseJSON{
		BaseRequest: &BaseRequest{
			Uin:      "li.Wxuin",
			Sid:      "li.Wxsid",
			Skey:     "li.Skey",
			DeviceID: fmt.Sprintf("e%d", time.Now().Unix()),
		},
		SyncKey: &SyncKey{
			Count: 2,
			List: []map[string]string{
				{
					"Key": "1",
					"Val": "B",
				},
			},
		},
	}

	b, err := json.Marshal(bj)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	log.Printf("b: %s", string(b))
}
