package browser

import (
	"os"
	"path/filepath"
)

// writeFile 写入文件,先创建父目录。供截图等产物使用。
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
