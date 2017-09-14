package wechat

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"golang.org/x/net/publicsuffix"
)

func newClient() *http.Client {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		log.Fatal(err)
	}
	return &http.Client{
		Timeout: time.Second * 30,
		Jar:     jar,
	}
}

func TestGetUUID(t *testing.T) {
	client := newClient()
	uuid, err := GetUUID(client)
	if err != nil {
		t.Fatalf("GetUUID failed: %v", err)
	}
	if uuid == "" {
		t.Errorf("Invalid uuid: %s", uuid)
	}
	log.Printf("uuid: %s", uuid)
}

func TestGetQRCode(t *testing.T) {
	client := newClient()
	uuid, err := GetUUID(client)
	if err != nil {
		t.Fatalf("GetUUID failed: %v", err)
	}
	log.Printf("uuid: %s", uuid)
	tmp, err := ioutil.TempFile("", "QR")
	if err != nil {
		t.Fatalf("TempFile failed: %v", err)
	}
	defer tmp.Close()
	if err := GetQRCode(client, uuid, tmp); err != nil {
		t.Errorf("GetQRCode failed; %v", err)
	}
}

func TestWaitUntilLoggedIn(t *testing.T) {
	client := newClient()
	uuid, err := GetUUID(client)
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
	if err := GetQRCode(client, uuid, tmp); err != nil {
		t.Fatalf("GetQRCode failed; %v", err)
	}
	log.Printf("Please scan %s from your phone\n", tmp.Name())
	uri, err := WaitUntilLoggedIn(client, uuid)
	if err != nil {
		t.Fatalf("WaitUntilLoggedIn failed: %v", err)
	}
	log.Printf("uri: %s", uri)
}

func TestInitialize(t *testing.T) {
	client := newClient()
	uuid, err := GetUUID(client)
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
	if err := GetQRCode(client, uuid, tmp); err != nil {
		t.Fatalf("GetQRCode failed; %v", err)
	}
	log.Printf("Please scan %s from your phone\n", tmp.Name())
	uri, err := WaitUntilLoggedIn(client, uuid)
	if err != nil {
		t.Fatalf("WaitUntilLoggedIn failed: %v", err)
	}
	log.Printf("uri: %s", uri)

	if err := Initialize(client, uri); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
}
