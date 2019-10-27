package main

import (
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

type session struct {
	ws          *websocket.Conn
	rl          *readline.Instance
	errChan     chan error
	pingHandler func(string) error
	pongHandler func(string) error
}

func connect(url, origin string, authHeader string, rlConf *readline.Config, allowInsecure bool) error {
	headers := make(http.Header)
	headers.Add("Origin", origin)
	if authHeader != "" {
		headers.Add("Authorization", authHeader)
	}

	dialer := websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: allowInsecure,
		},
	}
	ws, _, err := dialer.Dial(url, headers)
	if err != nil {
		return err
	}

	rl, err := readline.NewEx(rlConf)
	if err != nil {
		return err
	}
	defer rl.Close()

	sess := &session{
		ws:          ws,
		rl:          rl,
		errChan:     make(chan error),
		pingHandler: ws.PingHandler(),
		pongHandler: ws.PongHandler(),
	}

	ws.SetPongHandler(func(appData string) error {
		rxSprintf := color.New(color.FgGreen).SprintfFunc()
		fmt.Fprint(sess.rl.Stdout(), rxSprintf("< %s\n", "< PONG"))
		return sess.pongHandler(appData)
	})

	ws.SetPingHandler(func(appData string) error {
		rxSprintf := color.New(color.FgGreen).SprintfFunc()
		fmt.Fprint(sess.rl.Stdout(), rxSprintf("< %s\n", "> PING"))
		return sess.pingHandler(appData)
	})

	go sess.readConsole()
	go sess.readWebsocket()

	return <-sess.errChan
}

func (s *session) readConsole() {
	for {
		line, err := s.rl.Readline()
		if err != nil {
			s.errChan <- err
			return
		}

		if line == "ping" {
			err = s.ws.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				s.errChan <- err
				return
			}
		} else {

			err = s.ws.WriteMessage(websocket.TextMessage, []byte(line))
			if err != nil {
				s.errChan <- err
				return
			}
		}
	}
}

func bytesToFormattedHex(bytes []byte) string {
	text := hex.EncodeToString(bytes)
	return regexp.MustCompile("(..)").ReplaceAllString(text, "$1 ")
}

func (s *session) readWebsocket() {
	rxSprintf := color.New(color.FgGreen).SprintfFunc()

	for {
		msgType, buf, err := s.ws.ReadMessage()
		if err != nil {
			s.errChan <- err
			return
		}

		var text string
		switch msgType {
		case websocket.TextMessage:
			text = string(buf)
		case websocket.BinaryMessage:
			text = bytesToFormattedHex(buf)
		default:
			s.errChan <- fmt.Errorf("unknown websocket frame type: %d", msgType)
			return
		}

		fmt.Fprint(s.rl.Stdout(), rxSprintf("< %s\n", text))
	}
}
