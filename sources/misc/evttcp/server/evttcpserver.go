package evttcpserver

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
)

// ITCPServer defines the external API for the TCP server.
type ITCPServer interface {
	//Start(port int) error
	SetOnCLientConnect(f func(cli ITCPClient))
	SetOnClientDisconnect(f func(cli ITCPClient))
	SetOnClientData(f func(cli ITCPClient, data []byte))
	SetOnClientDataString(f func(cli ITCPClient, data string))
	DisconnectClient(cli ITCPClient) error
	Send(cli ITCPClient, data []byte) error
	SendString(cli ITCPClient, data string) error
	//SetTLSConfig(cfg *tls.Config)
	//EnableTLSFromFiles(certFile, keyFile string) error
	//DisableTLS()
}

// Option is used to configure the server at construction time.
// Return an error from an option to abort construction.
type Option func(*TCPServer) error

// WithTLSConfig returns an Option that sets a tls.Config on the server.
func WithTLSConfig(cfg *tls.Config) Option {
	return func(s *TCPServer) error {
		s.tlsConfig = cfg
		return nil
	}
}

// WithTLSFromFiles returns an Option that loads cert/key and configures TLS.
func WithTLSFromFiles(certFile, keyFile string) Option {
	return func(s *TCPServer) error {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}
		if s.tlsConfig == nil {
			s.tlsConfig = &tls.Config{}
		}
		s.tlsConfig.Certificates = []tls.Certificate{cert}
		return nil
	}
}

type TCPServer struct {
	OnClientData       func(cli *TCPClient, data []byte)
	OnClientDataString func(cli *TCPClient, data string)
	OnClientConnect    func(cli *TCPClient)
	OnClientDisconnect func(cli *TCPClient)
	// Optional TLS configuration. If set, the server will accept TLS connections.
	tlsConfig *tls.Config
	port      int
	// If set, listen on unix socket instead of TCP
	unixSocketPath string
}

// New constructs a TCP server. Pass zero or more Option values to configure TLS or other
// settings at construction time. Returns an ITCPServer (interface) and any error from
// applying options or starting the default listener.
func New(opts ...Option) (ITCPServer, error) {
	r := TCPServer{
		OnClientData:       func(cli *TCPClient, data []byte) {},
		OnClientDataString: func(cli *TCPClient, data string) {},
		OnClientConnect:    func(cli *TCPClient) {},
		OnClientDisconnect: func(cli *TCPClient) {},
		port:               9001,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(&r); err != nil {
			return nil, err
		}
	}

	// Start listening using configured options (port or unix socket).
	if err := r.init(); err != nil {
		return nil, err
	}
	return &r, nil
}

// WithPort returns an Option that sets the listening port for the TCP server.
func WithPort(port int) Option {
	return func(s *TCPServer) error {
		if port <= 0 || port > 65535 {
			return fmt.Errorf("invalid port %d", port)
		}
		s.port = port
		return nil
	}
}

// WithUnixSocket configures the server to listen on the provided unix domain socket path.
// If set, it takes precedence over the TCP port setting.
func WithUnixSocket(path string) Option {
	return func(s *TCPServer) error {
		if path == "" {
			return fmt.Errorf("empty unix socket path")
		}
		s.unixSocketPath = path
		return nil
	}
}

func (t *TCPServer) SetOnCLientConnect(f func(cli ITCPClient)) {
	t.OnClientConnect = func(c *TCPClient) {
		f(c)
	}
}

// SetOnClientDisconnect sets the callback that will be called when a client disconnects.
func (t *TCPServer) SetOnClientDisconnect(f func(cli ITCPClient)) {
	t.OnClientDisconnect = func(c *TCPClient) {
		f(c)
	}
}

// DisconnectClient forcefully closes the provided client's connection. This will
// cause the client's read loop to exit and the OnClientDisconnect handler to be
// invoked once the connection is fully closed.
func (t *TCPServer) DisconnectClient(cli ITCPClient) error {
	if cli == nil {
		return errors.New("nil client")
	}
	return cli.Disconnect()
}

func (t *TCPServer) SetOnClientData(f func(cli ITCPClient, data []byte)) {
	t.OnClientData = func(c *TCPClient, data []byte) {
		f(c, data)
	}
}

func (t *TCPServer) SetOnClientDataString(f func(cli ITCPClient, data string)) {
	t.OnClientDataString = func(c *TCPClient, data string) {
		f(c, data)
	}
}

func (t *TCPServer) init() error {
	var l net.Listener
	var err error
	if t.unixSocketPath != "" {
		// remove previous socket if present
		_ = os.Remove(t.unixSocketPath)
		l, err = net.Listen("unix", t.unixSocketPath)
		if err != nil {
			return errors.New("error starting unix socket server: " + err.Error())
		}
	} else {
		l, err = net.Listen("tcp", ":"+strconv.Itoa(t.port))
		if err != nil {
			return errors.New("error starting tcp server: " + err.Error())
		}
	}

	// If TLS configuration was provided and we're using TCP, wrap the listener to serve TLS.
	if t.tlsConfig != nil && t.unixSocketPath == "" {
		l = tls.NewListener(l, t.tlsConfig)
	}

	go t.work(l)
	return nil
}

func (t *TCPServer) work(server net.Listener) {
	defer func() {
		server.Close()
		if t.unixSocketPath != "" {
			_ = os.Remove(t.unixSocketPath)
		}
	}()

	for {
		conn, err := server.Accept()
		if err != nil {
			fmt.Println("Accept client error")
		}

		fmt.Println("Client accepted")

		go t.handleClient(conn)
	}
}

func (t *TCPServer) handleClient(con net.Conn) {
	cli := t.createCli(con)

	t.OnClientConnect(cli)

	buf := make([]byte, 1024*10)
	for {
		reqLen, err := con.Read(buf)

		if err != nil {
			break
		}

		if reqLen > 0 {
			t.processReceivedData(cli, buf, reqLen)
		}

	}

	//disconnected client
	t.OnClientDisconnect(cli)
}

func (t *TCPServer) processReceivedData(cli *TCPClient, buf []byte, reqLen int) {
	cli.processReceivedData(buf[0:reqLen])
	t.OnClientData(cli, buf[0:reqLen])
	t.OnClientDataString(cli, string(buf[0:reqLen]))
}

func (t *TCPServer) Send(cli ITCPClient, data []byte) error {
	if cli == nil {
		return errors.New("nil client")
	}
	cli.Send(data)
	return nil
}

func (t *TCPServer) SendString(cli ITCPClient, data string) error {
	if cli == nil {
		return errors.New("nil client")
	}
	cli.SendString(data)
	return nil
}

func (t *TCPServer) createCli(con net.Conn) *TCPClient {
	r := NewClientHelper(func(data []byte, sender TCPClient) error {
		_, err := con.Write(data)
		if err != nil {
			err = fmt.Errorf("error sending data to client: %w", err)
			return err
		}

		return nil
	}, func(sender TCPClient) error {
		return sender.Close()
	}, con)

	return &r
}
