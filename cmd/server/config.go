package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	addr            string
	dataDir         string
	selftest        bool
	selftestTimeout time.Duration
}

func parseConfig() (config, error) {
	value := config{}
	flag.StringVar(&value.addr, "addr", "127.0.0.1:19081", "HTTP 监听地址")
	flag.StringVar(&value.dataDir, "data-dir", "./data", "持久化数据目录")
	flag.BoolVar(&value.selftest, "selftest", false, "运行真实 HTTP 全流程自检后退出")
	flag.DurationVar(&value.selftestTimeout, "selftest-timeout", 15*time.Second, "自检总超时")
	flag.Parse()
	if value.addr == "127.0.0.1:19081" {
		if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
			port, err := strconv.Atoi(raw)
			if err != nil || port < 1 || port > 65535 {
				return value, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
			}
			value.addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	host, port, err := net.SplitHostPort(value.addr)
	if err != nil {
		return value, fmt.Errorf("-addr 必须是 host:port：%w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return value, fmt.Errorf("监听地址必须使用回环主机，拒绝 %s", host)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return value, fmt.Errorf("监听端口无效")
	}
	if value.selftestTimeout <= 0 {
		return value, fmt.Errorf("-selftest-timeout 必须大于 0")
	}
	return value, nil
}
