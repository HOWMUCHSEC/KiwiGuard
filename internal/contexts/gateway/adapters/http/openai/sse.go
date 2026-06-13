package openai

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
)

const maxSSEFrameBytes = 1 << 20

var errSSEFrameTooLarge = errors.New("sse frame too large")

// SSEFrame contains one parsed server-sent event frame.
type SSEFrame struct {
	Event   string
	Data    string
	ID      string
	Retry   string
	Comment string
	Done    bool
	Raw     []byte
}

// SSEParser reads server-sent event frames from an OpenAI-compatible stream.
type SSEParser struct {
	reader *bufio.Reader
	done   bool
}

// NewSSEParser builds an incremental parser for server-sent event frames.
func NewSSEParser(r io.Reader) *SSEParser {
	return &SSEParser{reader: bufio.NewReader(r)}
}

// Next returns the next SSE frame, or io.EOF after the final frame is consumed.
func (p *SSEParser) Next() (SSEFrame, error) {
	if p.done {
		return SSEFrame{}, io.EOF
	}

	var raw bytes.Buffer
	var lines []string
	for {
		line, err := p.reader.ReadString('\n')
		if len(line) > 0 {
			raw.WriteString(line)
			if raw.Len() > maxSSEFrameBytes {
				return SSEFrame{}, errSSEFrameTooLarge
			}
			trimmed := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			if trimmed == "" {
				break
			}
			lines = append(lines, trimmed)
		}
		if err != nil {
			if err == io.EOF && len(lines) > 0 {
				p.done = true
				break
			}
			if err == io.EOF {
				p.done = true
			}
			return SSEFrame{}, err
		}
	}

	frame := parseSSELines(lines)
	frame.Raw = raw.Bytes()
	if frame.Done {
		p.done = true
	}
	return frame, nil
}

func parseSSELines(lines []string) SSEFrame {
	var frame SSEFrame
	dataParts := make([]string, 0, len(lines))
	commentParts := make([]string, 0, 1)
	for _, line := range lines {
		if strings.HasPrefix(line, ":") {
			commentParts = append(commentParts, strings.TrimPrefix(strings.TrimPrefix(line, ":"), " "))
			continue
		}

		field, value, ok := strings.Cut(line, ":")
		if !ok {
			field = line
			value = ""
		} else if strings.HasPrefix(value, " ") {
			value = strings.TrimPrefix(value, " ")
		}

		switch field {
		case "event":
			frame.Event = value
		case "data":
			dataParts = append(dataParts, value)
		case "id":
			frame.ID = value
		case "retry":
			frame.Retry = value
		}
	}
	frame.Data = strings.Join(dataParts, "\n")
	frame.Comment = strings.Join(commentParts, "\n")
	frame.Done = strings.TrimSpace(frame.Data) == "[DONE]"
	return frame
}
