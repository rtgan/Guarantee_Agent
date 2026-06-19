package main

import (
	"os"

	"guarantee-agent/internal/cli"
)

// main 是 autoqa CLI 进程入口,把控制权交给 cli.Execute 并以其返回值退出。
func main() {
	os.Exit(cli.Execute())
}
