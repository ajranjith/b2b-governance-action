package mcpio

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxMessageSize = 10 * 1024 * 1024 // 10MB

// ReadMessage reads a Content-Length framed JSON-RPC message from r.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "content-length:") {
			val := strings.TrimSpace(line[len("content-length:"):])
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid Content-Length: %q", val)
			}
			if n > maxMessageSize {
				return nil, fmt.Errorf("content length %d exceeds limit %d", n, maxMessageSize)
			}
			contentLength = n
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	return body, nil
}

// WriteMessage writes a Content-Length framed JSON-RPC message to w.
func WriteMessage(w io.Writer, payload []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
