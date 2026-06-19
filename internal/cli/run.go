package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"guarantee-agent/internal/runner"
)

// newRunCommand 构建 `autoqa run <file-or-dir>` 命令。
func newRunCommand() *cobra.Command {
	var url, envName string
	var headless, debug bool
	cmd := &cobra.Command{
		Use:   "run <file-or-dir>",
		Short: "Run Markdown specs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := runner.Run(context.Background(), runner.RunOptions{Path: args[0], BaseURL: url, EnvName: envName, Headless: headless, Debug: debug, CWD: cwd()})
			if err != nil {
				// 配置/环境类错误退出码 2。
				return exitError{ExitConfig, err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "runId=%s total=%d passed=%d failed=%d artifacts=%s\n", s.RunID, s.Total, s.Passed, s.Failed, s.RunDir)
			// 有用例失败时退出码 1。
			if s.Failed > 0 {
				return exitError{ExitRuntime, fmt.Errorf("%d spec(s) failed", s.Failed)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "base URL")
	cmd.Flags().StringVar(&envName, "env", "", "environment name for .env.<name>")
	cmd.Flags().BoolVar(&headless, "headless", true, "run headless")
	cmd.Flags().BoolVar(&debug, "debug", false, "debug mode")
	return cmd
}
