package transport

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 4096)
	},
}

func ReadFrame(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}

	if length == 0 {
		return "", fmt.Errorf("empty frame")
	}

	if length > 65536 {
		return "", fmt.Errorf("frame too large: %d bytes", length)
	}

	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf[:0])

	if cap(buf) < int(length) {
		buf = make([]byte, length)
	} else {
		buf = buf[:length]
	}

	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

func WriteFrame(w io.Writer, hexPayload string) error {
	data := []byte(hexPayload)
	length := uint32(len(data))

	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}

	_, err := w.Write(data)
	return err
}
