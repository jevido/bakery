package podman

import (
	"bytes"
	"encoding/binary"
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
