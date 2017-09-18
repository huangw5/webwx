package wechat

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type stubClient struct {
	resp *http.Response
	err  error
}

func (s *stubClient) Do(method, url string, body io.Reader) (*http.Response, error) {
	return s.resp, s.err
}

type readerCloser struct {
	reader io.Reader
}

func (r *readerCloser) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *readerCloser) Close() error {
	return nil
}

func TestGetUUID(t *testing.T) {
	w := &Wechat{Client: NewClient()}
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
	w := &Wechat{Client: NewClient()}
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
	w := &Wechat{Client: NewClient()}
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

func TestInit(t *testing.T) {
	w := &Wechat{Client: NewClient()}
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

	if _, err := w.Init(uri); err != nil {
		t.Fatalf("Init failed: %v", err)
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
			Count: 1,
			List: []map[string]int{
				{
					"Key": 1,
					"Val": 3,
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

func TestSyncKey(t *testing.T) {
	sk := &SyncKey{
		Count: 2,
		List: []map[string]int{
			{
				"Key": 1,
				"Val": 100,
			},
			{
				"Key": 2,
				"Val": 200,
			},
			{
				"Key": 3,
				"Val": 300,
			},
		},
	}
	want := "1_100|2_200|3_300"
	if got := sk.String(); got != want {
		t.Errorf("got: %s, want %s", got, want)
	}
}

func TestSyncCheck(t *testing.T) {
	body := `window.synccheck={retcode:"1101",selector:"0"}`
	c := &stubClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:       &readerCloser{reader: strings.NewReader(body)},
		},
		err: nil,
	}
	w := &Wechat{Client: c}
	w.BaseJSON = &BaseJSON{
		BaseRequest: &BaseRequest{},
		SyncKey:     &SyncKey{},
	}
	sr, err := w.SyncCheck()
	if err != nil {
		t.Fatalf("SyncCheck failed: %v", err)
	}
	if sr.Retcode != "1101" {
		t.Errorf("Retcode = %s, want %s", sr.Retcode, "1101")
	}
	if sr.Selector != "0" {
		t.Errorf("Selector = %s, want %s", sr.Selector, "0")
	}
}
