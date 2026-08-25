package main

import (
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

const (
	baud          = 115200
	readTimeout   = 300 * time.Millisecond
	handshakeTO   = 6 * time.Second
	handshakeStep = 300 * time.Millisecond
	autoCooldown  = 8 * time.Second
)

var validKeys = map[string]bool{}

func init() {
	for i := 1; i <= 12; i++ {
		validKeys[fmt.Sprintf("F%d", i)] = true
	}
	validKeys["FAV"] = true
	validKeys["SEARCH"] = true
	for c := 'A'; c <= 'Z'; c++ {
		validKeys[string(c)] = true
	}
}

// ---------- 端口信息 ----------
type PortInfo struct {
	Name    string `json:"name"`
	Product string `json:"product"`
	Vid     string `json:"vid"`
	Pid     string `json:"pid"`
	Serial  string `json:"serial"`
}

func toPortInfo(d *enumerator.PortDetails) PortInfo {
	return PortInfo{Name: d.Name, Product: d.Product, Serial: d.SerialNumber, Vid: d.VID, Pid: d.PID}
}

func scanPorts() []PortInfo {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil
	}
	out := make([]PortInfo, 0, len(details))
	seen := map[string]bool{}
	for _, d := range details {
		if d == nil {
			continue
		}
		// 只列 USB 串口，避免干扰
		if !d.IsUSB {
			continue
		}
		if seen[d.Name] {
			continue
		}
		seen[d.Name] = true
		out = append(out, toPortInfo(d))
	}
	return out
}

func isCandidate(p PortInfo) bool {
	low := strings.ToLower(p.Product)
	if strings.Contains(low, "micro") || strings.Contains(low, "leonardo") {
		return true
	}
	if p.Vid == "2341" {
		return true
	}
	return false
}

// ---------- 应用状态 ----------
type Mapping struct {
	Knob   string `json:"knob"`
	Button string `json:"button"`
}

type App struct {
	mu         sync.Mutex
	dev        serial.Port
	portName   string
	connected  bool
	mapping    Mapping
	log        []string
	readerStop chan struct{}
	pongCh     chan struct{}
	failPort   string
	failAt     time.Time
}

func (a *App) appendLog(format string, args ...interface{}) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.log = append(a.log, line)
	if len(a.log) > 500 {
		a.log = a.log[len(a.log)-500:]
	}
}

func (a *App) currentMapping() Mapping {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mapping
}

// ---------- 串口连接 ----------
func (a *App) connect(portName string) error {
	a.mu.Lock()
	if a.connected && a.portName == portName {
		a.mu.Unlock()
		return nil
	}
	if a.connected {
		a.teardownLocked()
	}
	mode := &serial.Mode{BaudRate: baud}
	p, err := serial.Open(portName, mode)
	if err != nil {
		a.mu.Unlock()
		return fmt.Errorf("打开 %s 失败: %v", portName, err)
	}
	if err := p.SetReadTimeout(readTimeout); err != nil {
		a.mu.Unlock()
		_ = p.Close()
		return fmt.Errorf("设置 %s 超时失败: %v", portName, err)
	}
	a.dev = p
	a.portName = portName
	a.readerStop = make(chan struct{})
	a.pongCh = make(chan struct{}, 1)
	a.connected = true
	a.mu.Unlock()

	a.startReader()
	a.appendLog("已打开 %s，握手确认中...", portName)

	if err := a.handshake(); err != nil {
		a.teardown(err.Error())
		return err
	}
	a.appendLog("握手成功，读取当前映射")
	a.readMapping()
	a.saveLastPort(portName)
	a.mu.Lock()
	a.failPort = ""
	a.mu.Unlock()
	return nil
}

func (a *App) teardownLocked() {
	// 先置断开标记再关闭端口，避免读取协程在关闭瞬间误判为意外断开。
	a.connected = false
	a.portName = ""
	if a.readerStop != nil {
		close(a.readerStop)
		a.readerStop = nil
	}
	if a.dev != nil {
		_ = a.dev.Close()
		a.dev = nil
	}
}

func (a *App) teardown(reason string) {
	a.mu.Lock()
	a.teardownLocked()
	a.mu.Unlock()
	a.appendLog("连接断开: %s", reason)
}

func (a *App) handshake() error {
	a.mu.Lock()
	pongCh := a.pongCh
	dev := a.dev
	a.mu.Unlock()

	// Pro Micro 打开串口时会自动复位重枚举，因此要持续重试等它重新上线。
	deadline := time.Now().Add(handshakeTO)
	for {
		if _, err := dev.Write([]byte("PING\n")); err != nil {
			return err
		}
		select {
		case <-pongCh:
			return nil
		case <-time.After(handshakeStep):
		}
		if time.Now().After(deadline) {
			return errors.New("板子无响应（请先烧录 MicroOri 固件，且串口未被其他程序占用）")
		}
	}
}

func (a *App) startReader() {
	a.mu.Lock()
	dev := a.dev
	stop := a.readerStop
	pongCh := a.pongCh
	a.mu.Unlock()

	go func() {
		buf := make([]byte, 256)
		line := ""
		for {
			select {
			case <-stop:
				return
			default:
			}
			n, err := dev.Read(buf)
			if err != nil {
				// 端口关闭/设备拔出等真正的 IO 错误 -> 触发断开与自动重连。
				a.mu.Lock()
				stillConnected := a.connected
				a.mu.Unlock()
				if stillConnected {
					a.teardown("端口读取失败: " + err.Error())
				}
				return
			}
			if n == 0 {
				// Windows 驱动在空闲读超时时返回 (0, nil)，视为暂无数据继续等待。
				continue
			}
			for i := 0; i < n; i++ {
				b := buf[i]
				if b == '\n' {
					a.onLine(line, pongCh)
					line = ""
				} else if b != '\r' {
					line += string(b)
					if len(line) > 512 {
						line = ""
					}
				}
			}
		}
	}()
}

func (a *App) onLine(line string, pongCh chan struct{}) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	switch {
	case line == "PONG":
		select {
		case pongCh <- struct{}{}:
		default:
		}
	case strings.HasPrefix(line, "MAP knob "):
		a.mu.Lock()
		a.mapping.Knob = strings.TrimSpace(strings.TrimPrefix(line, "MAP knob "))
		a.mu.Unlock()
	case strings.HasPrefix(line, "MAP button "):
		a.mu.Lock()
		a.mapping.Button = strings.TrimSpace(strings.TrimPrefix(line, "MAP button "))
		a.mu.Unlock()
	}
	a.appendLog("<< %s", line)
}

func (a *App) writeLine(cmd string) error {
	a.mu.Lock()
	dev := a.dev
	if dev == nil {
		a.mu.Unlock()
		return errors.New("未连接")
	}
	a.mu.Unlock()
	_, err := dev.Write([]byte(cmd + "\n"))
	a.appendLog(">> %s", cmd)
	return err
}

func (a *App) readMapping() {
	_ = a.writeLine("GET")
	// 给板子一点时间回复
	time.Sleep(250 * time.Millisecond)
}

func (a *App) setMapping(m Mapping) error {
	if !validKeys[m.Knob] {
		return fmt.Errorf("无效按键: %s", m.Knob)
	}
	if !validKeys[m.Button] {
		return fmt.Errorf("无效按键: %s", m.Button)
	}
	if err := a.writeLine("SET knob " + m.Knob); err != nil {
		return err
	}
	time.Sleep(80 * time.Millisecond)
	if err := a.writeLine("SET button " + m.Button); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	a.mu.Lock()
	a.mapping = m
	a.mu.Unlock()
	a.appendLog("映射已保存到板子: 旋钮=%s 按钮=%s", m.Knob, m.Button)
	return nil
}

func (a *App) resetMapping() error {
	if err := a.writeLine("RESET"); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	a.readMapping()
	return nil
}

// ---------- 最近端口持久化 ----------
func configPath() string {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "microori.json")
}

type persist struct {
	LastPort string `json:"lastPort"`
}

func (a *App) loadLastPort() string {
	b, err := os.ReadFile(configPath())
	if err != nil {
		return ""
	}
	var p persist
	if json.Unmarshal(b, &p) == nil {
		return p.LastPort
	}
	return ""
}

func (a *App) saveLastPort(port string) {
	p := persist{LastPort: port}
	b, _ := json.Marshal(p)
	_ = os.WriteFile(configPath(), b, 0644)
}

// ---------- 后台自动连接 ----------
func (a *App) autoManager() {
	for {
		time.Sleep(2 * time.Second)
		a.mu.Lock()
		connected := a.connected
		a.mu.Unlock()
		if connected {
			continue
		}
		last := a.loadLastPort()
		ports := scanPorts()
		var candidate *PortInfo
		for i := range ports {
			if ports[i].Name == last {
				candidate = &ports[i]
				break
			}
		}
		if candidate == nil {
			for i := range ports {
				if isCandidate(ports[i]) {
					candidate = &ports[i]
					break
				}
			}
		}
		if candidate == nil && len(ports) == 1 {
			candidate = &ports[0]
		}
		if candidate != nil {
			a.mu.Lock()
			cooldownActive := candidate.Name == a.failPort && time.Since(a.failAt) < autoCooldown
			a.mu.Unlock()
			if cooldownActive {
				continue
			}
			if err := a.connect(candidate.Name); err != nil {
				a.mu.Lock()
				a.failPort = candidate.Name
				a.failAt = time.Now()
				a.mu.Unlock()
				a.appendLog("自动连接 %s 失败: %v", candidate.Name, err)
			}
		}
	}
}

// ---------- HTTP ----------
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonBody(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "请求格式错误"})
		return false
	}
	return true
}

func main() {
	app := &App{mapping: Mapping{Knob: "F8", Button: "F9"}}

	keyOptions := []map[string]string{}
	for i := 1; i <= 12; i++ {
		v := fmt.Sprintf("F%d", i)
		keyOptions = append(keyOptions, map[string]string{"value": v, "label": v, "group": "F1-F12"})
	}
	keyOptions = append(keyOptions,
		map[string]string{"value": "FAV", "label": "浏览器收藏", "group": "浏览器"},
		map[string]string{"value": "SEARCH", "label": "浏览器搜索", "group": "浏览器"},
	)
	for c := 'A'; c <= 'Z'; c++ {
		keyOptions = append(keyOptions, map[string]string{"value": string(c), "label": string(c), "group": "字母 A-Z"})
	}

	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		app.mu.Lock()
		defer app.mu.Unlock()
		writeJSON(w, map[string]interface{}{
			"ok":        true,
			"connected": app.connected,
			"port":      app.portName,
			"mapping":   app.mapping,
			"log":       app.log,
		})
	})

	http.HandleFunc("/api/options", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"ok": true, "options": keyOptions})
	})

	http.HandleFunc("/api/ports", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"ok": true, "ports": scanPorts()})
	})

	http.HandleFunc("/api/connect", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Port string `json:"port"`
		}
		if !jsonBody(w, r, &req) {
			return
		}
		if req.Port == "" {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "未指定端口"})
			return
		}
		if err := app.connect(req.Port); err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true})
	})

	http.HandleFunc("/api/connect-auto", func(w http.ResponseWriter, r *http.Request) {
		last := app.loadLastPort()
		ports := scanPorts()
		var target string
		for _, p := range ports {
			if p.Name == last {
				target = p.Name
				break
			}
		}
		if target == "" {
			for _, p := range ports {
				if isCandidate(p) {
					target = p.Name
					break
				}
			}
		}
		if target == "" && len(ports) == 1 {
			target = ports[0].Name
		}
		if target == "" {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "没有找到 MicroOri 板子，请插好并刷新"})
			return
		}
		if err := app.connect(target); err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "port": target})
	})

	http.HandleFunc("/api/disconnect", func(w http.ResponseWriter, r *http.Request) {
		app.teardown("手动断开")
		writeJSON(w, map[string]interface{}{"ok": true})
	})

	http.HandleFunc("/api/mapping", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			app.readMapping()
			writeJSON(w, map[string]interface{}{"ok": true, "mapping": app.currentMapping()})
		case http.MethodPost:
			var m Mapping
			if !jsonBody(w, r, &m) {
				return
			}
			if err := app.setMapping(m); err != nil {
				writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{"ok": true})
		default:
			writeJSON(w, map[string]interface{}{"ok": false, "error": "method"})
		}
	})

	http.HandleFunc("/api/reset", func(w http.ResponseWriter, r *http.Request) {
		if err := app.resetMapping(); err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true})
	})

	// 前端页面打包进 exe，单文件运行。
	filesys, err := iofs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("加载内置页面失败: %v", err)
	}
	fs := http.FileServer(http.FS(filesys))
	http.Handle("/", fs)

	// 随机端口，避免端口被占用；界面由内置窗口加载，无需打开浏览器。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("启动本地服务失败: %v", err)
	}
	serveURL := "http://" + ln.Addr().String()
	log.Printf("MicroOri 本地服务: %s", serveURL)

	go func() {
		if err := http.Serve(ln, nil); err != nil && err != http.ErrServerClosed {
			log.Printf("本地服务异常: %v", err)
		}
	}()

	go app.autoManager()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "MicroOri",
			Width:  960,
			Height: 760,
			Center: true,
		},
	})
	if w == nil {
		_, _ = windows.MessageBox(
			windows.HWND(0),
			windows.StringToUTF16Ptr("WebView2 runtime is required.\nPlease install Microsoft Edge WebView2 Runtime."),
			windows.StringToUTF16Ptr("MicroOri"),
			windows.MB_ICONERROR,
		)
		return
	}
	defer w.Destroy()
	w.Navigate(serveURL + "/")
	w.Run()
}
