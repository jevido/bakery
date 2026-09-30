package podman

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

// demux reads the multiplexed log stream of a container without a TTY:
// frames of an 8-byte header (stream type, 3 zero bytes, big-endian length)
// followed by that many bytes. Frames can split a line, so partial lines are
// held per stream until their newline arrives.
func demux(r io.Reader, out func(stream, line string)) error {
	br := bufio.NewReader(r)
	var header [8]byte
	pending := map[string]string{}
	flush := func() {
		for stream, rest := range pending {
			if rest != "" {
				out(stream, rest)
			}
		}
	}
	for {
		if _, err := io.ReadFull(br, header[:]); err != nil {
			flush()
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		stream := "stdout"
		if header[0] == 2 {
			stream = "stderr"
		}
		size := binary.BigEndian.Uint32(header[4:])
		payload := make([]byte, size)
		if _, err := io.ReadFull(br, payload); err != nil {
			flush()
			return err
		}
		text := pending[stream] + string(payload)
		lines := strings.Split(text, "\n")
		for _, line := range lines[:len(lines)-1] {
			out(stream, strings.TrimRight(line, "\r"))
		}
		pending[stream] = lines[len(lines)-1]
	}
}

// demuxTo copies the frames of a multiplexed stream unchanged to stdout or
// stderr, for binary output such as a database dump.
func demuxTo(r io.Reader, stdout, stderr io.Writer) error {
	br := bufio.NewReaderSize(r, 64<<10)
	var header [8]byte
	for {
		if _, err := io.ReadFull(br, header[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		w := stdout
		if header[0] == 2 {
			w = stderr
		}
		if _, err := io.CopyN(w, br, int64(binary.BigEndian.Uint32(header[4:]))); err != nil {
			return err
		}
	}
}
