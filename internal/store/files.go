package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// InvalidBatchID 表示批次标识包含存储路径不允许的字符。
// 该错误属于客户端可修正的 validation 级问题，不应归类为内部错误。
type InvalidBatchID struct{ Reason string }

func (e *InvalidBatchID) Error() string { return e.Reason }

// ValidateBatchID 校验批次标识是否只包含存储路径允许的字符，供应用层在触碰持久化前调用。
func ValidateBatchID(value string) error {
	_, err := safeID(value)
	return err
}

func safeID(value string) (string, error) {
	if value == "" || value == "." || value == ".." {
		return "", &InvalidBatchID{Reason: "非法批次标识"}
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return "", &InvalidBatchID{Reason: "批次标识只能包含字母、数字、连字符和下划线"}
		}
	}
	return value, nil
}

func atomicJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".commit-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o640); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	parent, err := os.Open(dir)
	if err == nil {
		err = parent.Sync()
		parent.Close()
	}
	return err
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}
