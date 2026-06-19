package ir

import (
	"bufio"
	"encoding/json"
	"os"
)

// ReadFile 读取一个 ir.jsonl 文件,逐行反序列化为 ActionRecord 切片。空行被跳过。
func ReadFile(path string) ([]ActionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []ActionRecord
	s := bufio.NewScanner(f)
	for s.Scan() {
		if len(s.Bytes()) == 0 {
			continue
		}
		var rec ActionRecord
		if err := json.Unmarshal(s.Bytes(), &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, s.Err()
}
