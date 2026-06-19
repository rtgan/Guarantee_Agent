package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"guarantee-agent/internal/config"
)

// newInitCommand 构建 `autoqa init` 命令,创建配置、示例用例和 steps 目录。
func newInitCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create AutoQA config and example spec",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cwd()
			cfgPath := filepath.Join(root, "autoqa.config.json")
			if force {
				_ = os.Remove(cfgPath)
			}
			if err := config.WriteDefault(cfgPath); err != nil {
				return exitError{ExitConfig, err}
			}
			if err := os.MkdirAll(filepath.Join(root, "specs"), 0755); err != nil {
				return exitError{ExitConfig, err}
			}
			// 不覆盖已存在的示例用例。
			example := filepath.Join(root, "specs", "example.md")
			if _, err := os.Stat(example); os.IsNotExist(err) {
				_ = os.WriteFile(example, []byte(exampleSpec), 0644)
			}
			if err := os.MkdirAll(filepath.Join(root, "steps"), 0755); err != nil {
				return exitError{ExitConfig, err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", cfgPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config")
	return cmd
}

// exampleSpec 是 init 写出的示例 Markdown 用例。
const exampleSpec = `# Example

## Preconditions
- The test fixture server or target application is available.

## Steps
1. Navigate to {{BASE_URL}}
   - Expected: the page loads
2. Verify Example text is visible
`
