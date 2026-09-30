package podman

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"testing"
)

func frame(stream byte, s string) []byte {
	h := make([]byte, 8)
	h[0] = stream
	binary.BigEndian.PutUint32(h[4:], uint32(len(s)))
	return append(h, s...)
}

func TestDemux(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(frame(1, "hello\nwor"))
	buf.Write(frame(2, "oops\n"))
	buf.Write(frame(1, "ld\nlast"))
	var got []string
	if err := demux(&buf, func(stream, line string) { got = append(got, stream+":"+line) }); err != nil {
		t.Fatal(err)
	}
	want := []string{"stdout:hello", "stderr:oops", "stdout:world", "stdout:last"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestDemuxTo(t *testing.T) {
	big := make([]byte, 200<<10)
	for i := range big {
		big[i] = byte(rand.IntN(256))
	}
	var in bytes.Buffer
	in.Write(frame(1, "a\x00b\n"))
	in.Write(frame(2, "warning\n"))
	in.Write(frame(1, string(big)))
	var stdout, stderr bytes.Buffer
	if err := demuxTo(&in, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if want := append([]byte("a\x00b\n"), big...); !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("stdout: %d bytes, want %d", stdout.Len(), len(want))
	}
	if stderr.String() != "warning\n" {
		t.Fatalf("stderr %q", stderr.String())
	}
	// A frame cut short is an error, not a silently shorter dump.
	cut := frame(1, "abcdef")
	if err := demuxTo(bytes.NewReader(cut[:10]), &stdout, &stderr); err == nil {
		t.Fatal("truncated frame: no error")
	}
}
