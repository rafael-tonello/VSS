package evttcpclient

import (
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"syscall"
)

type TCPClient struct {
	OnDataString func(data string)
	OnData       func(data []byte, size int)
	OnConnect    func()
	OnDisconnect func()
	con          net.Conn

	sendingMutex sync.Mutex

	Tags map[string]string
}

// Instantiate a new TCPClient to be used to connect to a server. You need to use the 'Connect' function to make that connection.
func NewTCPClient() *TCPClient {
	r := TCPClient{}
	r.OnData = func(dt []byte, size int) {}
	r.OnDataString = func(dt string) {}
	r.Tags = map[string]string{}
	r.OnConnect = func() {}
	r.OnDisconnect = func() {}

	return &r
}

// This constructor is uded by TCPServer to use this class as a helper to talk with clients
func newClientServerHelper(con net.Conn) *TCPClient {
	r := TCPClient{}
	r.OnData = nil
	r.OnDataString = nil
	r.con = con
	r.Tags = map[string]string{}

	return &r
}

// a very useful function to send data in a string format
func (this *TCPClient) SendString(data string) error {
	return this.Send([]byte(data))
}

// send data to server
func (this *TCPClient) Send(data []byte) error {
	this.sendingMutex.Lock()
	defer this.sendingMutex.Unlock()

	if this.con == nil {
		return errors.New("connection is not stablished")
	} else {
		_, err := this.con.Write(data)
		return err
	}
}

// this function processes data incoming from server
func (this *TCPClient) processReceivedData(data []byte, size int) {
	if this.OnData != nil {
		this.OnData(data[0:size], size)
	}

	if this.OnDataString != nil {
		this.OnDataString(string(data[0:size]))
	}
}

func (this *TCPClient) readDataProcess(processDataFunc func(data []byte, size int)) {
	this.OnConnect()

	buf := make([]byte, 1024*10)
	for {
		reqLen, err := this.con.Read(buf)

		if err != nil {
			break
		}

		if reqLen > 0 {
			processDataFunc(buf, reqLen)
		}

	}

	//disconnected client
	this.OnDisconnect()
}

// connects to a TCP Server. The TCPCLient will attempt to connect to the server 'serverAddress' on port 'serverPort'.
func (this *TCPClient) connect(serverAndPortString string) error {
	con, err := net.Dial("tcp", serverAndPortString)
	if err != nil {
		return err
	}

	this.con = con

	go this.readDataProcess(func(data []byte, size int) {
		this.processReceivedData(data, size)
	})

	return nil
}

// Try connect to the server. If connection is stablished with sucess, 'nil' will be
// returnted (this function returns an error, or nil). Otherwise, the connection error
// will be returned
func (this *TCPClient) Connect(option func(*TCPClient) error) error {
	return option(this)
}

func (this *TCPClient) Disconnect() error {
	return this.con.Close()
}

func (this *TCPClient) IsConnected() bool {
	return connCheck(this.con) == nil
}

func ConnectWithServerAndPort(server string, port int) func(*TCPClient) error {
	return func(cli *TCPClient) error {
		cli.connect(server + ":" + strconv.Itoa(port))
		return nil
	}
}

// connets to the server using a string in the format 'server:port'
func ConnectWithString(conString string) func(*TCPClient) error {
	return func(cli *TCPClient) error {
		return cli.connect(conString)
	}
}

// Check if the net.Conn is connected
// Code taken from: https://stackoverflow.com/a/58664631
func connCheck(conn net.Conn) error {
	var sysErr error = nil
	rc, err := conn.(syscall.Conn).SyscallConn()
	if err != nil {
		return err
	}
	err = rc.Read(func(fd uintptr) bool {
		var buf []byte = []byte{0}
		n, _, err := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		switch {
		case n == 0 && err == nil:
			sysErr = io.EOF
		case err == syscall.EAGAIN || err == syscall.EWOULDBLOCK:
			sysErr = nil
		default:
			sysErr = err
		}
		return true
	})
	if err != nil {
		return err
	}

	return sysErr
}
