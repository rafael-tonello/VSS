package evttcpserver

import "net"

type ITCPClient interface {
	SendString(data string) error
	Send(data []byte) error
	Disconnect() error
}

type TCPClient struct {
	OnDataString       func(data string)
	OnData             func(data []byte)
	con                net.Conn
	sendFunction       func(data []byte, sender TCPClient) error
	disconnectFunction func(sender TCPClient) error

	Tags map[string]string
}

func NewClientHelper(sendFunction func(data []byte, sender TCPClient) error, disconnectFunction func(sender TCPClient) error, con net.Conn) TCPClient {
	r := TCPClient{}
	r.OnData = nil
	r.OnDataString = nil
	r.con = con
	r.sendFunction = sendFunction
	r.disconnectFunction = disconnectFunction
	r.Tags = map[string]string{}

	return r
}

func (ch TCPClient) SendString(data string) error {
	return ch.Send([]byte(data))
}

func (ch TCPClient) Send(data []byte) error {
	return ch.sendFunction(data, ch)
}

func (ch TCPClient) Disconnect() error {
	return ch.disconnectFunction(ch)
}

func (ch TCPClient) processReceivedData(data []byte) {
	if ch.OnData != nil {
		ch.OnData(data)
	}

	if ch.OnDataString != nil {
		ch.OnDataString(string(data))
	}
}

// Close disconnects the client by closing the underlying connection.
// It is safe to call multiple times; subsequent calls return nil.
func (ch *TCPClient) Close() error {
	if ch == nil || ch.con == nil {
		return nil
	}
	err := ch.con.Close()
	ch.con = nil
	return err
}
