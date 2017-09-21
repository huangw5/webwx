package wechat

import (
	"fmt"
	"strings"
)

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
	User        *Member      `json:"User"`
}

// BaseResponse is.
type BaseResponse struct {
	Ret    int    `json:"Ret"`
	ErrMsg string `json:"ErrMsg"`
}

// AddMsg is new message.
type AddMsg struct {
	MsgID        string `json:"MsgId"`
	MsgType      int    `json:"MsgType"`
	Content      string `json:"Content"`
	FromUserName string `json:"FromUserName"`
	NickName     string
}

// Member is contact.
type Member struct {
	UserName string `json:"UserName"`
	NickName string `json:"NickName"`
}

// BaseResponseJSON is.
type BaseResponseJSON struct {
	BaseResponse *BaseResponse `json:"BaseResponse"`
	AddMsgCount  int           `json:"AddMsgCount"`
	AddMsgList   []*AddMsg     `json:"AddMsgList"`
	MemberList   []*Member     `json:"MemberList"`
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
