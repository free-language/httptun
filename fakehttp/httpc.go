package fakehttp

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"io/ioutil"
	"time"
)

var (
	errNotServer       = errors.New("may not tunnel server")
)

type Client struct {
	TxMethod      string
	RxMethod      string
	TxFlag        string
	RxFlag        string
	TokenCookieA  string
	TokenCookieB  string
	TokenCookieC  string
	UserAgent     string
	Url           string
	Timeout       time.Duration
	Host          string
	UseWs         bool
}

func (cl *Client) getToken() (string, error) {
	client := &http.Client{
		Timeout: cl.Timeout,
	}

	req, err := http.NewRequest("GET", "http://" + cl.Host + cl.Url, nil)
	if err != nil {
		Vlogln(2, "getToken() NewRequest err:", err)
		return "", err
	}

	req.Header.Set("User-Agent", cl.UserAgent)
	res, err := client.Do(req)
	if err != nil {
		Vlogln(2, "getToken() send Request err:", err)
		return "", err
	}
	defer res.Body.Close()

	cookies := res.Cookies()

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		Vlogln(2, "getToken() ReadAll err:", err)
	}
	Vlogln(3, "getToken()", cookies)

	for _, cookie := range cookies {
		Vlogln(3, "cookie:", cookie.Name, cookie.Value)
		if cookie.Name == cl.TokenCookieA {
			return cookie.Value, nil
		}
	}

	return  "", errNotServer
}

func (cl *Client) getTx(token string) (net.Conn, []byte, error) { //io.WriteCloser

	req, err := http.NewRequest(cl.TxMethod, "http://" + cl.Host, nil)
	if err != nil {
		Vlogln(2, "getTx() NewRequest err:", err)
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "private, no-store, no-cache, max-age=0")
	req.Header.Set("User-Agent", cl.UserAgent)
	req.Header.Set("Cookie", cl.TokenCookieB + "=" + token + "; " + cl.TokenCookieC + "=" + cl.TxFlag)
	if cl.UseWs {
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Key", token)
		req.Header.Set("Sec-WebSocket-Version", "13")
	}


	tx, err := net.DialTimeout("tcp", cl.Host, cl.Timeout)
	if err != nil {
		Vlogln(2, "Tx connect to:", cl.Host, err)
		return nil, nil, err
	}
	req.Write(tx)
	Vlogln(3, "Tx connect ok:", cl.Host)

	txbuf := bufio.NewReaderSize(tx, 1024)
//	Vlogln(2, "Tx Reader", txbuf)
	res, err := http.ReadResponse(txbuf, req)
	if err != nil {
		Vlogln(2, "Tx ReadResponse", err, res)
		return nil, nil, err
	}
	n := txbuf.Buffered()
	Vlogln(3, "Tx Response", n)

	return tx, nil, nil
}

func (cl *Client) getRx(token string) (net.Conn, []byte, error) { //io.ReadCloser

	req, err := http.NewRequest(cl.RxMethod, "http://" + cl.Host, nil)
	if err != nil {
		Vlogln(2, "getRx() NewRequest err:", err)
		return nil, nil, err
	}

	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "private, no-store, no-cache, max-age=0")
	req.Header.Set("User-Agent", cl.UserAgent)
	req.Header.Set("Cookie", cl.TokenCookieB + "=" + token + "; " + cl.TokenCookieC + "=" + cl.RxFlag)
	if cl.UseWs {
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Key", token)
		req.Header.Set("Sec-WebSocket-Version", "13")
	}

	rx, err := net.DialTimeout("tcp", cl.Host, cl.Timeout)
	if err != nil {
		Vlogln(2, "Rx connect to:", cl.Host, err)
		return nil, nil, err
	}
	Vlogln(3, "Rx connect ok:", cl.Host)
	req.Write(rx)

	rxbuf := bufio.NewReaderSize(rx, 1024)
//	Vlogln(2, "Rx Reader", rxbuf)
	res, err := http.ReadResponse(rxbuf, req)
	if err != nil {
		Vlogln(2, "Rx ReadResponse", err, res, rxbuf)
		return nil, nil, err
	}

	n := rxbuf.Buffered()
	Vlogln(3, "Rx Response", n)
	if n > 0 {
		buf := make([]byte, n)
		rxbuf.Read(buf[:n])
		return rx, buf[:n], nil
	} else {
		return rx, nil, nil
	}
}


func NewClient(target string) (*Client) {
	cl := &Client {
		TxMethod:     txMethod,
		RxMethod:     rxMethod,
		TxFlag:       txFlag,
		RxFlag:       rxFlag,
		TokenCookieA: tokenCookieA,
		TokenCookieB: tokenCookieB,
		TokenCookieC: tokenCookieC,
		UserAgent:    userAgent,
		Url:          targetUrl,
		Timeout:      timeout,
		Host:         target,
		UseWs:        false,
	}
	return cl
}

func Dial(target string) (net.Conn, error) {
	cl := NewClient(target)
	return cl.Dial()
}

func (cl *Client) Dial() (net.Conn, error) {
	token, err := cl.getToken()
	if token == "" || err != nil {
		return nil, err
	}
	Vlogln(2, "token:", token)

	tx, _, err := cl.getTx(token)
	if err != nil {
		return nil, err
	}
	Vlogln(4, "tx:", tx)

	rx, rxbuf, err := cl.getRx(token)
	if err != nil {
		return nil, err
	}
	Vlogln(4, "rx:", rx, rxbuf)

	return mkconn(rx, tx, rxbuf), nil
}
