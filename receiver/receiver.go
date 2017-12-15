package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"
)

// Config for server
type Config struct {
	LogPath       string
	DebugLog      int
	Port          int
	ReadTimeoutMs time.Duration
}

var config Config

func ref(arg interface{}) {
	// nop
}

func onLog(format string, a ...interface{}) {
	if config.DebugLog != 0 {
		fmt.Print(time.Now().Format("[2006-01-02 15:04:05 Z07:00] "))
		fmt.Printf(format, a...)
	}
}

func onLogln(a ...interface{}) {
	if config.DebugLog != 0 {
		fmt.Print(time.Now().Format("[2006-01-02 15:04:05 Z07:00] "))
		fmt.Println(a...)
	}
}

func onError(err error) {
	if config.DebugLog != 0 {
		fmt.Print(time.Now().Format("[2006-01-02 15:04:05 Z07:00] "))
		fmt.Println(err)
	}
}

// shell makes user interactive environment
func shell(handler func(string) bool) {
	getCommand := func() string {
		var cmd string
		fmt.Scanln(&cmd)
		return cmd
	}
	for handler(getCommand()) {
	}
}

// getExitHandler returns default handler that handles exit command.
func getExitHandler(handler func(string) bool) func(string) bool {
	return func(cmd string) bool {
		if cmd == "exit" {
			return false
		}
		return handler(cmd)
	}
}

func commandHandler(cmd string) bool {
	switch cmd {
	case "":
		// nop
	default:
		onLog("unknown command!!!")
	}
	return true
}

func newTicker(d time.Duration, handler func(*time.Ticker, time.Time)) *time.Ticker {
	ticker := time.NewTicker(d)
	go func() {
		for t := range ticker.C {
			handler(ticker, t)
		}
	}()
	return ticker
}

func callAndGetWatchHandler(filename string, handler func(filename string)) func(ticker *time.Ticker, t time.Time) {
	initialStat, err := os.Stat(filename)
	if err != nil {
		onError(err)
		return nil
	}
	handler(filename)
	return func(ticker *time.Ticker, t time.Time) {
		if stat, err := os.Stat(filename); err == nil {
			if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
				initialStat = stat
				handler(filename)
			}
		} else {
			onError(err)
		}
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		onLog("Incoming connection closed. (%s, %s)\n", conn.RemoteAddr().Network(), conn.RemoteAddr().String())
		conn.Close()
	}()

	onLog("New connection established. (%s, %s)\n", conn.RemoteAddr().Network(), conn.RemoteAddr().String())

	timeout := config.ReadTimeoutMs * time.Millisecond
	bufReader := bufio.NewReader(conn)

	for {
		conn.SetReadDeadline(time.Now().Add(timeout))

		if b, err := bufReader.ReadByte(); err == nil {
			onLog("%b", b)
		} else {
			onLog("\n")
			if err != io.EOF {
				onError(err)
			}
			break
		}
	}
}

func main() {
	watcher := newTicker(time.Second, callAndGetWatchHandler("./config.json", func(filename string) {
		if b, err := ioutil.ReadFile(filename); err == nil {
			json.Unmarshal(b, &config)
			if config.Port > 0 {
				go func() {
					if ln, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port)); err == nil {
						for {
							if conn, err := ln.Accept(); err == nil {
								go handleConnection(conn)
							} else {
								onError(err)
							}
						}
					} else {
						onError(err)
					}
				}()
			}
		} else {
			onError(err)
			panic("not found config file")
		}
	}))

	shell(getExitHandler(commandHandler))
	if watcher != nil {
		watcher.Stop()
	}
}
