package main

import (
	"os"

	"guarantee-agent/internal/cli"
)

// main 是 autoqa CLI 进程入口
func main() {
	// os.Exit(n) 的含义是：拿着退出码n结束进程。这个码会传给调用方（shell、CI、父进程），它们据此判断「成功还是失败、失败是哪一类」
	os.Exit(cli.Execute())
}
