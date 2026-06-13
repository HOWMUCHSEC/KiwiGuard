package application

import (
	"bytes"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
)

// StreamBuffer keeps transport-neutral streaming output state for policy checks.
type StreamBuffer struct {
	window    StreamWindow
	collector StreamTextCollector
}

// NewStreamBuffer creates streaming state with bounded detection and capture buffers.
func NewStreamBuffer(windowLimit int, collectionLimit int64) *StreamBuffer {
	return &StreamBuffer{
		window:    StreamWindow{limit: windowLimit},
		collector: StreamTextCollector{limit: collectionLimit},
	}
}

// ObserveDelta advances both the detection window and retained output buffer for one streamed delta.
func (b *StreamBuffer) ObserveDelta(text string) StreamDeltaObservation {
	if b == nil {
		return StreamDeltaObservation{}
	}
	b.window.Append(text)
	return StreamDeltaObservation{
		WindowText:         b.window.Text(),
		CollectionAccepted: b.collector.Append(text),
	}
}

// CollectedText exposes the retained output text accumulated for final response handling.
func (b *StreamBuffer) CollectedText() string {
	if b == nil {
		return ""
	}
	return b.collector.Text()
}

// CollectedBytes exposes the retained output bytes accumulated for final response handling.
func (b *StreamBuffer) CollectedBytes() []byte {
	if b == nil {
		return nil
	}
	return b.collector.Bytes()
}

// StreamDeltaObservation contains transport-neutral state after one streamed text delta.
type StreamDeltaObservation struct {
	WindowText         string
	CollectionAccepted bool
}

// StreamTextCollector keeps complete stream text until a byte budget is exceeded.
type StreamTextCollector struct {
	limit int64
	buf   bytes.Buffer
}

// NewStreamTextCollector builds a collector that retains streaming text up to a byte budget.
func NewStreamTextCollector(limit int64) *StreamTextCollector {
	return &StreamTextCollector{limit: limit}
}

// Append adds text and reports whether it still fits within the collector budget.
func (c *StreamTextCollector) Append(text string) bool {
	if c == nil || c.limit <= 0 {
		return false
	}
	if int64(c.buf.Len()+len(text)) > c.limit {
		return false
	}
	c.buf.WriteString(text)
	return true
}

// Text exposes the collector's retained stream text.
func (c *StreamTextCollector) Text() string {
	if c == nil {
		return ""
	}
	return c.buf.String()
}

// Bytes exposes the collector's retained stream text as bytes.
func (c *StreamTextCollector) Bytes() []byte {
	if c == nil {
		return nil
	}
	return c.buf.Bytes()
}

// StreamWindow keeps a bounded text suffix for incremental stream detection.
type StreamWindow struct {
	limit int
	text  []rune
}

// NewStreamWindow builds a sliding text window bounded by rune count.
func NewStreamWindow(limit int) *StreamWindow {
	return &StreamWindow{limit: limit}
}

// Append adds text to the window, discarding the oldest runes past the limit.
func (w *StreamWindow) Append(text string) {
	if w == nil {
		return
	}
	if w.limit <= 0 {
		w.text = nil
		return
	}

	w.text = append(w.text, []rune(text)...)
	if len(w.text) > w.limit {
		w.text = w.text[len(w.text)-w.limit:]
	}
}

// Text exposes the current rune-bounded detection window.
func (w *StreamWindow) Text() string {
	if w == nil {
		return ""
	}
	return string(w.text)
}

// Match runs a detector against the retained text.
func (w *StreamWindow) Match(detector detection.Detector, direction detection.Direction) []detection.Finding {
	return detector.Match(detection.Input{
		Direction: direction,
		Text:      w.Text(),
	})
}
