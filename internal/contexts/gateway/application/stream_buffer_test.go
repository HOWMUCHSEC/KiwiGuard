package application

import (
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
)

func TestStreamBufferObservesDeltas(t *testing.T) {
	buffer := NewStreamBuffer(5, 16)

	first := buffer.ObserveDelta("hello")
	if first.WindowText != "hello" || !first.CollectionAccepted {
		t.Fatalf("first observation = %+v, want complete accepted window", first)
	}

	second := buffer.ObserveDelta(" world")
	if second.WindowText != "world" || !second.CollectionAccepted {
		t.Fatalf("second observation = %+v, want bounded suffix", second)
	}
	if got := buffer.CollectedText(); got != "hello world" {
		t.Fatalf("CollectedText() = %q, want %q", got, "hello world")
	}
	if got := string(buffer.CollectedBytes()); got != "hello world" {
		t.Fatalf("CollectedBytes() = %q, want %q", got, "hello world")
	}
}

func TestStreamBufferRejectsOversizedCollection(t *testing.T) {
	buffer := NewStreamBuffer(64, 5)

	if got := buffer.ObserveDelta("hello"); !got.CollectionAccepted {
		t.Fatalf("ObserveDelta(hello).CollectionAccepted = false, want true")
	}
	if got := buffer.ObserveDelta("!"); got.CollectionAccepted {
		t.Fatalf("ObserveDelta(!).CollectionAccepted = true, want false")
	}
	if got := buffer.CollectedText(); got != "hello" {
		t.Fatalf("CollectedText() = %q, want retained accepted text", got)
	}
}

func TestStreamWindowFindsPIIAcrossChunks(t *testing.T) {
	window := NewStreamWindow(64)
	detector, err := detection.Compile(detection.Definition{Key: "email", Kind: detection.KindEmail})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	window.Append("alice@")
	if got := window.Match(detector, detection.DirectionOutput); len(got) != 0 {
		t.Fatalf("Match(before complete email) = %d findings, want 0", len(got))
	}
	window.Append("example.com")
	if got := window.Match(detector, detection.DirectionOutput); len(got) != 1 {
		t.Fatalf("Match(after complete email) = %d findings, want 1", len(got))
	}
}

func TestStreamWindowBoundsTextByRunes(t *testing.T) {
	window := NewStreamWindow(5)

	window.Append("hello")
	window.Append("世界")

	if got := window.Text(); got != "llo世界" {
		t.Fatalf("Text() = %q, want %q", got, "llo世界")
	}
}

func TestStreamWindowZeroLimitClearsText(t *testing.T) {
	window := NewStreamWindow(0)

	window.Append("secret")

	if got := window.Text(); got != "" {
		t.Fatalf("Text() = %q, want empty", got)
	}
}

func TestStreamTextCollectorBytesAndNilReceivers(t *testing.T) {
	collector := NewStreamTextCollector(8)
	if !collector.Append("kiwi") {
		t.Fatal("Append(kiwi) = false, want true")
	}
	if got := string(collector.Bytes()); got != "kiwi" {
		t.Fatalf("Bytes() = %q, want kiwi", got)
	}

	var nilBuffer *StreamBuffer
	if got := nilBuffer.ObserveDelta("ignored"); got != (StreamDeltaObservation{}) {
		t.Fatalf("nil ObserveDelta() = %+v, want zero observation", got)
	}
	if got := nilBuffer.CollectedText(); got != "" {
		t.Fatalf("nil CollectedText() = %q, want empty", got)
	}
	if got := nilBuffer.CollectedBytes(); got != nil {
		t.Fatalf("nil CollectedBytes() = %#v, want nil", got)
	}

	var nilCollector *StreamTextCollector
	if nilCollector.Append("ignored") {
		t.Fatal("nil Append() = true, want false")
	}
	if got := nilCollector.Text(); got != "" {
		t.Fatalf("nil Text() = %q, want empty", got)
	}
	if got := nilCollector.Bytes(); got != nil {
		t.Fatalf("nil Bytes() = %#v, want nil", got)
	}

	zeroLimit := NewStreamTextCollector(0)
	if zeroLimit.Append("ignored") {
		t.Fatal("zero-limit Append() = true, want false")
	}
	if got := zeroLimit.Text(); got != "" {
		t.Fatalf("zero-limit Text() = %q, want empty", got)
	}
}

func TestStreamWindowNilReceiverIsNoop(t *testing.T) {
	var window *StreamWindow

	window.Append("ignored")
	if got := window.Text(); got != "" {
		t.Fatalf("nil window Text() = %q, want empty", got)
	}
}
