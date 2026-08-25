package main

import "testing"

func TestAddressConfiguration(t *testing.T) {
	t.Setenv("PORT", "19444")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != "127.0.0.1:19444" {
		t.Fatalf("PORT 未生效：%s", cfg.addr)
	}
	cfg, err = parseConfig([]string{"-addr=127.0.0.1:19555"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != "127.0.0.1:19555" {
		t.Fatalf("显式 addr 未优先：%s", cfg.addr)
	}
	if _, err = parseConfig([]string{"-addr=:19081"}); err == nil {
		t.Fatal("应拒绝缺少主机")
	}
	if _, err = parseConfig([]string{"-addr=0.0.0.0:19081"}); err == nil {
		t.Fatal("应拒绝通配地址")
	}
}
