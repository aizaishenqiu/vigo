package port

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// CheckAndKill 检查端口是否被占用，如果被占用则杀掉占用进程
// 返回 true 表示端口可用（原本空闲或已成功释放）
func CheckAndKill(portNum int, serviceName string) bool {
	if !IsPortInUse(portNum) {
		log.Printf("[端口检测] %s 端口 %d 空闲，可以使用", serviceName, portNum)
		return true
	}

	log.Printf("[端口检测] %s 端口 %d 被占用，尝试释放...", serviceName, portNum)

	pid := GetPIDByPort(portNum)
	if pid <= 0 {
		log.Printf("[端口检测] 无法找到占用端口 %d 的进程", portNum)
		return false
	}

	log.Printf("[端口检测] 端口 %d 被进程 PID=%d 占用，正在终止...", portNum, pid)
	if err := KillProcess(pid); err != nil {
		log.Printf("[端口检测] 终止进程 PID=%d 失败: %v", pid, err)
		return false
	}

	// 等待端口释放
	for i := 0; i < 10; i++ {
		time.Sleep(200 * time.Millisecond)
		if !IsPortInUse(portNum) {
			log.Printf("[端口检测] 端口 %d 已成功释放", portNum)
			return true
		}
	}

	log.Printf("[端口检测] 端口 %d 释放超时", portNum)
	return false
}

// IsPortInUse 检查端口是否被占用
func IsPortInUse(portNum int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

// CheckAll 批量检查并释放端口
// ports: map[服务名]端口号
func CheckAll(ports map[string]int, autoKill bool) error {
	for name, p := range ports {
		if p <= 0 {
			continue
		}
		if IsPortInUse(p) {
			if autoKill {
				if !CheckAndKill(p, name) {
					return fmt.Errorf("%s 端口 %d 被占用且无法释放", name, p)
				}
			} else {
				return fmt.Errorf("%s 端口 %d 被占用，请手动释放或设置 auto_kill_port: true", name, p)
			}
		} else {
			log.Printf("[端口检测] %s 端口 %d 可用 ✓", name, p)
		}
	}
	return nil
}

// GetPIDByPort 根据端口号获取占用该端口的进程 PID
func GetPIDByPort(portNum int) int {
	switch runtime.GOOS {
	case "windows":
		return getPIDByPortWindows(portNum)
	default:
		return getPIDByPortUnix(portNum)
	}
}

// getPIDByPortWindows Windows 下通过 netstat 获取
func getPIDByPortWindows(portNum int) int {
	cmd := exec.Command("cmd", "/c", "netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return -1
	}

	portStr := fmt.Sprintf(":%d", portNum)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		if !strings.Contains(line, portStr) {
			continue
		}

		// 提取 PID（最后一列数字）
		re := regexp.MustCompile(`\s+(\d+)\s*$`)
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			pid, err := strconv.Atoi(matches[1])
			if err == nil && pid > 0 {
				return pid
			}
		}
	}
	return -1
}

// getPIDByPortUnix Linux/macOS 下通过 lsof 获取
func getPIDByPortUnix(portNum int) int {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", portNum), "-t")
	output, err := cmd.Output()
	if err != nil {
		return -1
	}

	pidStr := strings.TrimSpace(string(output))
	lines := strings.Split(pidStr, "\n")
	if len(lines) > 0 {
		pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
		if err == nil {
			return pid
		}
	}
	return -1
}

// KillProcess 杀掉指定 PID 的进程
func KillProcess(pid int) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
		return cmd.Run()
	default:
		cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
		return cmd.Run()
	}
}
