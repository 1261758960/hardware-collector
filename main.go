package main

import (
	"crypto/sha256"
	_ "embed" // 必须引入 embed 包用于打包 ico
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

// 允许的前端域名
// 生产环境请替换为: https://xjssjb.dscq.com
// 局域网联调时，请确保这里配置的是浏览器真实的 Origin，或者临时使用 "*"
const AllowedOrigin = "http://192.168.2.188:5173"

//go:embed icon.ico
var iconData []byte

type HardwareData struct {
	MacAddresses []string `json:"mac_addresses"`
	CPUID        string   `json:"cpu_id"`
	BoardSerial  string   `json:"board_serial"`
	DiskSerials  []string `json:"disk_serials"`
	InnerIP      string   `json:"inner_ip"`
	OuterIP      string   `json:"outer_ip"`
}

type ReportRequest struct {
	ProjectID  string       `json:"projectId"`
	SupplierID string       `json:"supplierId"`
	Hardware   HardwareData `json:"hardware"`
}

func hashString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// runCmd 执行系统命令，并隐藏弹出的黑框
func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)

	// 调用我们自定义的跨平台隐藏黑框函数 (在 hide_windows.go 和 hide_unix.go 中实现)
	hideCmdWindow(cmd)

	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func getInnerIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getOuterIP() string {
	client := http.Client{Timeout: 3 * time.Second}
	// 使用国内稳定的接口，返回纯文本 IP
	resp, err := client.Get("https://myip.ipip.net")
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		str := strings.TrimSpace(string(body))
		// myip.ipip.net 返回格式通常是: "当前 IP：114.24.x.x  来自于：..."
		if strings.Contains(str, "当前 IP：") {
			parts := strings.Split(str, "当前 IP：")
			if len(parts) > 1 {
				ip := strings.Split(parts[1], " ")[0]
				return strings.TrimSpace(ip)
			}
		}
		return str
	}

	// 备用接口
	resp2, err2 := client.Get("http://ip.3322.net")
	if err2 == nil && resp2.StatusCode == http.StatusOK {
		defer resp2.Body.Close()
		body, _ := io.ReadAll(resp2.Body)
		return strings.TrimSpace(string(body))
	}

	return ""
}

func collectHardware() HardwareData {
	data := HardwareData{MacAddresses: []string{}, DiskSerials: []string{}}
	data.InnerIP = getInnerIP()
	data.OuterIP = getOuterIP()

	if runtime.GOOS == "windows" {
		// ---------------- Windows 采集逻辑 ----------------
		// 1. 获取 MAC 地址
		macOut := runCmd("cmd", "/c", "getmac /fo csv /nh")
		lines := strings.Split(macOut, "\n")
		for _, line := range lines {
			parts := strings.Split(line, ",")
			if len(parts) > 0 {
				mac := strings.Trim(parts[0], "\" \r\n")
				if strings.Contains(mac, "-") {
					data.MacAddresses = append(data.MacAddresses, (mac))
					break
				}
			}
		}
		// 2. CPU ID
		cpuOut := runCmd("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_Processor).ProcessorId")
		cpuOut = strings.TrimSpace(cpuOut)
		if cpuOut != "" && !strings.Contains(cpuOut, "错误") {
			data.CPUID = (cpuOut)
		}
		// 3. 主板序列号
		boardOut := runCmd("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_BaseBoard).SerialNumber")
		boardOut = strings.TrimSpace(boardOut)
		if boardOut != "" && !strings.Contains(boardOut, "错误") && boardOut != "To be filled by O.E.M." {
			data.BoardSerial = (boardOut)
		}
		// 4. 硬盘序列号
		diskOut := runCmd("powershell", "-NoProfile", "-Command", "Get-PhysicalDisk | Select-Object -ExpandProperty SerialNumber")
		diskLines := strings.Split(diskOut, "\n")
		for _, line := range diskLines {
			serial := strings.TrimSpace(line)
			if serial != "" && !strings.Contains(serial, "错误") {
				data.DiskSerials = append(data.DiskSerials, (serial))
			}
		}

	} else if runtime.GOOS == "darwin" {
		// ---------------- macOS (苹果电脑) 采集逻辑 ----------------
		// 1. 获取 MAC 地址
		macOut := runCmd("sh", "-c", "ifconfig en0 | grep ether")
		if strings.Contains(macOut, "ether") {
			parts := strings.Fields(macOut)
			if len(parts) >= 2 {
				data.MacAddresses = append(data.MacAddresses, (parts[1]))
			}
		}

		// 2. CPU 编号 (Mac 通常使用 CPU 序列号或硬件 UUID 代替)
		cpuOut := runCmd("sh", "-c", "sysctl -n machdep.cpu.brand_string")
		cpuOut = strings.TrimSpace(cpuOut)
		if cpuOut != "" {
			data.CPUID = (cpuOut)
		}

		// 3. 主板/整机序列号 (Mac 的硬件唯一标识通常在 Hardware UUID 中)
		boardOut := runCmd("sh", "-c", "ioreg -rd1 -c IOPlatformExpertDevice | awk '/IOPlatformUUID/ { print $3 }'")
		boardOut = strings.Trim(boardOut, "\"\n\r ")
		if boardOut != "" {
			data.BoardSerial = (boardOut)
		}

		// 4. 硬盘序列号
		diskOut := runCmd("sh", "-c", "diskutil info / | grep 'Volume UUID' | awk -F: '{print $2}'")
		diskLines := strings.Split(diskOut, "\n")
		for _, line := range diskLines {
			serial := strings.TrimSpace(line)
			if serial != "" && serial != "N/A" {
				data.DiskSerials = append(data.DiskSerials, (serial))
			}
		}
	}

	return data
}

// 处理跨域
func setupCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", AllowedOrigin)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// 独立的 HTTP 服务器启动函数
func startServer() {
	mux := http.NewServeMux()

	// 探活接口
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// 采集接口
	mux.HandleFunc("/collect", func(w http.ResponseWriter, r *http.Request) {
		if setupCORS(w, r) {
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 1. 解析前端传来的项目信息
		var req ReportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// 2. 执行硬件采集
		req.Hardware = collectHardware()

		// 3. 去除了转发给 Java 后端的逻辑，直接组装数据返回给 Vue 前端
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"status":   "success",
			"message":  "数据采集成功",
			"hardware": req.Hardware, // 把采集到的数据返回给前端
		}
		json.NewEncoder(w).Encode(response)
	})

	// 启动服务，如果端口被占用会阻塞在这里
	err := http.ListenAndServe("127.0.0.1:38080", mux)
	if err != nil {
		// 如果启动失败，直接退出程序
		panic(err)
	}
}

// 托盘程序入口
func main() {
	// 在后台协程中启动 HTTP 服务
	go startServer()

	// 启动托盘图标 (阻塞主线程)
	systray.Run(onReady, onExit)
}

// 托盘启动时的回调
func onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("安全助手")
	systray.SetTooltip("开评标硬件环境采集助手正在运行")

	// 菜单项
	mStatus := systray.AddMenuItem("运行状态: 正常", "本地服务 127.0.0.1:38080")
	mStatus.Disable()
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出安全助手", "关闭采集服务并退出")

	// 监听退出按钮
	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	// 【新增】在后台弹出一个系统级的通知提示
	go showStartupNotification()
}

// 托盘退出时的回调
func onExit() {
	// 可以在这里做一些清理工作
}

// 【新增】展示启动成功的系统通知
// 【新增】展示启动成功的系统通知（改用弹窗，保证100%可见）
func showStartupNotification() {
	// 延迟 1 秒，等待托盘图标加载
	time.Sleep(1 * time.Second)

	if runtime.GOOS == "windows" {
		// 使用 Windows 自带的 mshta 弹出一个原生的提示框
		// 注意：这里不需要隐藏黑框，因为 mshta 本身就是无边框窗口
		cmd := exec.Command("mshta", `javascript:alert('安全助手已成功启动！\n\n硬件环境采集服务正在后台运行，请返回网页继续操作。');close();`)
		_ = cmd.Run()

	} else if runtime.GOOS == "darwin" {
		// macOS 同样使用 AppleScript 弹出原生弹窗
		script := `display dialog "安全助手已成功启动！\n\n硬件环境采集服务正在后台运行，请返回网页继续操作。" buttons {"确定"} with title "西南联交所" with icon note`
		runCmd("osascript", "-e", script)
	}
}
