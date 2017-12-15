package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"
)

// Remote for Config
type Remote struct {
	Host string
	Port int
}

// Config for server
type Config struct {
	IntervalMs  time.Duration
	CloseWaitMs time.Duration
	LogPath     string
	DebugLog    int
	Remote      Remote
	Message     []byte
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

func newTicker(d time.Duration, handler func(*time.Ticker)) *time.Ticker {
	ticker := time.NewTicker(d)
	go func() {
		for t := range ticker.C {
			ref(t)
			handler(ticker)
		}
	}()
	return ticker
}

func getFileWatchHandler(filename string, handler func(filename string)) func() {
	var initialStat os.FileInfo
	return func() {
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

func callAndGetHandler(handler func(*time.Ticker)) func(*time.Ticker) {
	handler(nil)
	return func(ticker *time.Ticker) {
		handler(ticker)
	}
}

func callAndGetFileWatchHandler(filename string, handler func(*time.Ticker)) func(*time.Ticker) {
	initialStat, err := os.Stat(filename)
	if err != nil {
		onError(err)
		return nil
	}
	handler(nil)
	return func(ticker *time.Ticker) {
		if stat, err := os.Stat(filename); err == nil {
			if stat.Size() != initialStat.Size() || stat.ModTime() != initialStat.ModTime() {
				initialStat = stat
				handler(ticker)
			}
		} else {
			onError(err)
		}
	}
}

func getFileLoader(filename string, onLoad func(*time.Ticker), onError func(error)) func(*time.Ticker) {
	return func(ticker *time.Ticker) {
		if b, err := ioutil.ReadFile(configFilename); err == nil {
			json.Unmarshal(b, &config)
			onLoad(ticker)
		} else {
			onError(err)
		}
	}
}

const configFilename = "./config.json"

func main() {
	var runner *time.Ticker
	watcher := newTicker(time.Second, callAndGetFileWatchHandler(configFilename, getFileLoader(configFilename, func(ticker *time.Ticker) {
		if config.IntervalMs <= 0 {
			return
		}
		if runner != nil {
			runner.Stop()
		}
		runner = newTicker(config.IntervalMs*time.Millisecond, callAndGetHandler(func(ticker *time.Ticker) {
			onLog("Connect to %s:%d\n", config.Remote.Host, config.Remote.Port)
			go func() {
				if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.Remote.Host, config.Remote.Port)); err == nil {
					onLogln("Send bytes:", config.Message)
					conn.Write(config.Message)
					time.Sleep(config.CloseWaitMs * time.Millisecond)
					conn.Close()
					onLogln("Done")
				} else {
					onError(err)
				}
			}()
		}))
	}, func(err error) {
		onError(err)
		panic("not found config file")
	})))

	shell(getExitHandler(commandHandler))
	if watcher != nil {
		watcher.Stop()
	}
	if runner != nil {
		runner.Stop()
	}
}
