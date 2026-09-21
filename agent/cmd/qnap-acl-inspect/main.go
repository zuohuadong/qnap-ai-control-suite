// qnap-acl-inspect only reads the native Windows detailed permission view.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"qnap-ai-control-suite/agent/internal/qnap/nativeacl"
)

func main() {
	base := flag.String("base-url", "", "NAS base URL")
	target := flag.String("target", "", "Exact shared-folder path to inspect")
	flag.Parse()
	// 会话仅从运行时环境读取，禁止放入命令行参数或输出。
	sid := os.Getenv("QNAP_NATIVE_SID")
	if *base == "" || *target == "" || sid == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Require --base-url, --target and QNAP_NATIVE_SID")
		os.Exit(2)
	}
	snapshot, err := nativeacl.ReadDetailed(context.Background(), *base, sid, *target, []string{*target})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(snapshot); err != nil {
		fmt.Fprintln(os.Stderr, "Cannot write native ACL snapshot")
		os.Exit(1)
	}
	// 成功读到部分信息不等于可以据此写入，使用不同退出码阻止流水线误用。
	if snapshot.Converting || !snapshot.Complete {
		fmt.Fprintln(os.Stderr, "Read-only evidence: conversion active or completeness unconfirmed; no writes permitted")
		os.Exit(3)
	}
}
