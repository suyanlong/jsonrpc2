package socket

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/jsonrpc2/json"
)

const MAXLEN = 32 * 1024 * 1024 // 32M

type ObjectStream struct {
	conn     net.Conn
	rw       *bufio.ReadWriter
	reChan   chan error
	isTrycon int32

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
		ip:       ip,
		port:     port,
		conn:     conn,
		rw:       rw,
		quit:     make(chan struct{}),
		reChan:   make(chan error, 1),
		isTrycon: 0,
	}
	go obj.TryReConnection()
	return obj, nil
}

func (o *ObjectStream) TryReConnection() {
	for {
		select {
		case <-o.quit:
			return
		case <-o.reChan:
			atomic.StoreInt32(&o.isTrycon, 1)
			err := o.TryConnection()
			if err != nil {
				log.Printf("TryReConnection: %s", err.Error())
			}
			atomic.StoreInt32(&o.isTrycon, 0)
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

func (o *ObjectStream) checkTryConnError(err error) error {
	_, ok := err.(*net.OpError)
	if (ok || err == io.EOF) && atomic.LoadInt32(&o.isTrycon) == 0 {
		o.reChan <- err
	}
	return err
}

func (o *ObjectStream) WriteObject(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	data = append(data, []byte("\n")...)
	_, err = o.rw.Write(data)
	if err != nil {
		return o.checkTryConnError(err)
	}
	err = o.rw.Flush()
	return o.checkTryConnError(err)
}

func (o *ObjectStream) ReadObject(v interface{}) error {
	data, _, err := o.rw.ReadLine()
	if err != nil {
		return o.checkTryConnError(err)
	}
	return json.Unmarshal(data, v)
}

func (o *ObjectStream) TryConnection() error {
	count := 0
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", o.ip, o.port), time.Second*10)
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
