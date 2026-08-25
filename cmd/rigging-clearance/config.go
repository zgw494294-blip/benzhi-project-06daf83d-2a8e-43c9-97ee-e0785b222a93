package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type config struct {
	addr, dataDir string
	selfcheck     bool
}

func parseConfig(args []string) (config, error) {
	set := flag.NewFlagSet("rigging-clearance", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	var cfg config
	set.StringVar(&cfg.addr, "addr", "127.0.0.1:19081", "HTTP 监听地址")
	set.StringVar(&cfg.dataDir, "data-dir", "./data", "事件数据目录")
	set.BoolVar(&cfg.selfcheck, "selfcheck", false, "通过真实 HTTP 监听器执行完整冒烟流程后退出")
	if err := set.Parse(args); err != nil {
		return cfg, err
	}
	if set.NArg() != 0 {
		return cfg, errors.New("不接受位置参数")
	}
	explicit := false
	set.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			explicit = true
		}
	})
	if !explicit {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return cfg, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
			}
			cfg.addr = net.JoinHostPort("127.0.0.1", port)
		}
	}
	if err := validateAddr(cfg.addr, cfg.selfcheck); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.dataDir) == "" {
		return cfg, errors.New("data-dir 不能为空")
	}
	cfg.dataDir = filepath.Clean(cfg.dataDir)
	return cfg, nil
}

func validateAddr(addr string, selfcheck bool) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("addr 必须为 host:port：%w", err)
	}
	host = strings.Trim(host, "[]")
	if strings.TrimSpace(host) == "" {
		return errors.New("addr 必须显式包含主机")
	}
	if host == "0.0.0.0" || host == "::" {
		return errors.New("拒绝监听通配地址，请显式指定安全主机")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return errors.New("addr 端口无效")
	}
	if selfcheck && host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("selfcheck 必须使用回环监听地址")
		}
	}
	return nil
}
