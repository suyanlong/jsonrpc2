package socket

import (
	"bufio"
	"bytes"
	"testing"
)

func TestWriteString(t *testing.T) {
	const BufSize = 1
	buf := new(bytes.Buffer)
	b := bufio.NewWriterSize(buf, BufSize)
	t.Log(b.WriteString("0"))                         // easy
	t.Log(b.WriteString("123456"))                    // still easy
	t.Log(b.WriteString("7890"))                      // easy after flush
	t.Log(b.WriteString("abcdefghijklmnopqrstuvwxy")) // hard
	t.Log(b.WriteString("z"))
	t.Log(b.Size())
	t.Log(b.Size())
	t.Log(b.Available())
	t.Log(b.Buffered())

	if err := b.Flush(); err != nil {
		t.Error("WriteString", err)
	}

	t.Log(b.Size())
	t.Log(b.Available())
	t.Log(b.Buffered())
	s := "01234567890abcdefghijklmnopqrstuvwxyz"
	if string(buf.Bytes()) != s {
		t.Errorf("WriteString wants %q gets %q", s, string(buf.Bytes()))
	}
}
