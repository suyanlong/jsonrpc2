package socket

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/sourcegraph/jsonrpc2/json"
)

const MAXLEN = 32 * 1024 * 1024 // 32M

//const HEADBEAT = `{"method":"wallet_status","id":1,"jsonrpc":"2.0"}`

type ObjectStream struct {
	conn net.Conn
	rw   *bufio.ReadWriter

	ip      string
	port    int
	timeOut time.Time
	quit    chan struct{}
}

func NewObjectStream(ip string, port int) (*ObjectStream, error) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReaderSize(conn, MAXLEN)
	wt := bufio.NewWriterSize(conn, MAXLEN)

	rw := bufio.NewReadWriter(rd, wt)

	obj := &ObjectStream{
		ip:   ip,
		port: port,
		conn: conn,
		rw:   rw,
		quit: make(chan struct{}),
	}

	return obj, nil
}

func (t *ObjectStream) HeartBeat(beatData string, heartBeat time.Duration) {
	ticker := time.NewTicker(heartBeat)
	data := append([]byte(beatData), []byte("\n")...)
	tryConn := func(err error) {
		_, ok := err.(*net.OpError)
		if ok {
			err := t.TryConnection()
			if err != nil {
				log.Println(err)
			}
		}
	}
	for {
		select {
		case <-t.quit:
			return
		case <-ticker.C:
			n, err := t.rw.Write([]byte(data))
			if n == 0{
				tryConn(err)
			}

			err = t.rw.Flush()
			if err != nil && err != io.ErrShortWrite {
				tryConn(err)
			}
		}
	}
}

func (o *ObjectStream) Close() error {
	err := o.conn.Close()
	if err != nil {
		return err
	}
	o.quit <- struct{}{}
	close(o.quit)
	return nil
}

func (o *ObjectStream) WriteObject(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	data = append(data, []byte("\n")...)
	_, err = o.rw.Write(data)
	if err != nil {
		return err
	}
	return o.rw.Flush()
}

func (o *ObjectStream) ReadObject(v interface{}) error {
	data, _, err := o.rw.ReadLine()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (o *ObjectStream) TryConnection() error {
	count := 0
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", o.ip, o.port),time.Second * 10)
		if err != nil {
			count++
			if count > 3 {
				return err
			}
			continue
		}

		o.conn = conn
		o.rw.Reader.Reset(conn)
		o.rw.Writer.Reset(conn)
		return nil
	}
}
