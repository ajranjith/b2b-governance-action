package flow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"
)

var ErrNotFound = errors.New("not found")

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

func decodeJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func newCommand(path string, args []string, timeout time.Duration) *exec.Cmd {
	if path == "" {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	return exec.CommandContext(ctx, path, args...)
}
