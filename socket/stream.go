package socket

import (
	"errors"
	"net"
	"time"

	"github.com/json-iterator/go"
	"github.com/valyala/bytebufferpool"
)

const MAXLEN = 16 * 1024 * 1024
const HEADBEAT = `{"method":"wallet_status","id":1,"jsonrpc":"2.0"}` + "\n"

type ObjectStream struct {
	conn   *TcpStream
	buffer *bytebufferpool.ByteBuffer
	quit   chan struct{}
}

func NewObjectStream(ip string, port int, timeOut time.Time) (*ObjectStream, error) {
	stream, err := NewTcpStream(ip, port, timeOut)
	if err != nil {
		return nil, err
	}

	obj := &ObjectStream{
		conn: stream,
		quit: make(chan struct{}),
	}
	go obj.headBeat()
	return obj, nil
}

func (t *ObjectStream) headBeat() {
	ticker := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-t.quit:
			return
		case <-ticker.C:
			_, err := t.conn.Write([]byte(HEADBEAT))
			if err != nil {
				err := t.conn.TryConnection()
				if err != nil {
					return
				}
			}
		}
	}
}

func (t *ObjectStream) Close() error {
	err := t.conn.Close()
	if err != nil {
		return err
	}

	t.quit <- struct{}{}
	close(t.quit)
	return nil
}

func (t *ObjectStream) WriteObject(obj interface{}) error {
	data, err := jsoniter.Marshal(obj)
	if err != nil {
		return err
	}

	data = append(data, []byte("\n")...)

	return t.Send(data)
}

func (t *ObjectStream) ReadObject(v interface{}) error {
	if t.buffer == nil {
		t.buffer = bytebufferpool.Get()
	}

	defer func() {
		bytebufferpool.Put(t.buffer)
		t.buffer = nil
	}()

	for {
		_, err := t.buffer.ReadFrom(t.conn)
		if err != nil {
			return nil
		}
		// TODO
		if t.buffer.Len() > MAXLEN {
			return errors.New("response data too many large")
		}

		l := t.buffer.Len()
		data := t.buffer.Bytes()

		if data[l-1] == 0x10 {
			return jsoniter.Unmarshal(data[:l-1], v)
		}
		for key, value := range data {
			if value == 0x10 {
				t.buffer.Set(data[key:])
				return jsoniter.Unmarshal(data[:key-1], v)
			}
		}
	}
}

func (t *ObjectStream) Send(data []byte) error {
	var start, c int
	var err error
	for {
		if c, err = t.conn.Write(data[start:]); err != nil {
			return err
		}
		start += c
		if c == 0 || start == len(data) {
			break
		}
	}
	return nil
}

type TcpStream struct {
	*net.TCPConn
	ip      string
	port    int
	timeOut time.Time
}

func NewTcpStream(ip string, port int, timeOut time.Time) (*TcpStream, error) {
	stream := &TcpStream{
		ip:      ip,
		port:    port,
		timeOut: timeOut,
	}
	err := stream.TryConnection()
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (t *TcpStream) TryConnection() error {
	count := 0
	for {
		conn, err := connection(t.ip, t.port, t.timeOut)
		if err != nil {
			count++
			if count > 3 {
				return err
			}
			continue
		}

		t.TCPConn = conn
		return nil
	}
}

func connection(ip string, port int, timeOut time.Time) (*net.TCPConn, error) {
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP(ip), Port: port})
	if err != nil {
		return nil, err
	}
	err = conn.SetKeepAlive(true)
	if err != nil {
		return nil, err
	}
	err = conn.SetReadDeadline(timeOut)
	if err != nil {
		return nil, err
	}
	err = conn.SetWriteDeadline(timeOut)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
