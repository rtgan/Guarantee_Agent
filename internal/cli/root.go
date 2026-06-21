package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// 退出码,与原项目语义保持一致。
const (
	ExitOK      = 0 // 成功
	ExitRuntime = 1 // 运行失败(用例执行失败)
	ExitConfig  = 2 // 用户/配置错误
)

// exitError 携带退出码与错误,供 cobra RunE 返回后由 Execute 翻译为进程退出码。
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }

// Execute 构建并运行根命令,把错误翻译为进程退出码。
func Execute() int {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		if ee, ok := err.(exitError); ok {
			fmt.Fprintln(os.Stderr, ee.err)
			return ee.code
		}
		fmt.Fprintln(os.Stderr, err)
		return ExitRuntime
	}
	return ExitOK
}

// newRootCommand 构建根命令并注册全部子命令。
func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "autoqa",
		Short:         "Run Markdown acceptance tests with a Go Eino agent",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newInitCommand(), newRunCommand(), newPlanCommand(), newPlanExploreCommand(), newPlanGenerateCommand())
	return cmd
}

// cwd 返回当前工作目录,出错时返回空串。
func cwd() string { dir, _ := os.Getwd(); return dir }
