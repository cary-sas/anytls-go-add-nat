package main

import (
	std_bufio "bufio"
	"context"
	"net"
	"runtime/debug"

	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
	"github.com/sirupsen/logrus"
)

type InboundListener struct {
	addr     string
	mode     string // "socks5" or "nat"
	client   *myClient
	listener net.Listener
}

func NewInboundListener(addr string, mode string, client *myClient) *InboundListener {
	return &InboundListener{
		addr:   addr,
		mode:   mode,
		client: client,
	}
}

func (il *InboundListener) Start(ctx context.Context) {
	var err error
	il.listener, err = net.Listen("tcp", il.addr)
	if err != nil {
		logrus.Fatalln("["+il.mode+"] listen error:", err)
	}
	defer il.listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := il.listener.Accept()
		if err != nil {
			logrus.Warnln("["+il.mode+"] accept error:", err)
			continue
		}

		go il.handleConnection(ctx, conn)
	}
}

func (il *InboundListener) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorln("[BUG]", r, string(debug.Stack()))
		}
	}()
	defer conn.Close()

	switch il.mode {
	case "socks5":
		il.handleSOCKS5(ctx, conn)
	case "nat":
		il.handleNAT(ctx, conn)
	}
}

// SOCKS5 handling logic
func (il *InboundListener) handleSOCKS5(ctx context.Context, conn net.Conn) {
	reader := std_bufio.NewReader(conn)
	headerBytes, err := reader.Peek(1)
	if err != nil {
		return
	}

	metadata := M.Metadata{
		Source:      M.SocksaddrFromNet(conn.RemoteAddr()),
		Destination: M.SocksaddrFromNet(conn.LocalAddr()),
	}

	switch headerBytes[0] {
	case socks4.Version, socks5.Version:
		socks.HandleConnection0(ctx, conn, reader, nil, il.client, metadata)
	default:
		http.HandleConnection(ctx, conn, reader, nil, il.client, metadata)
	}
}

// NAT handling logic (transparent forwarding, no protocol negotiation)
func (il *InboundListener) handleNAT(ctx context.Context, conn net.Conn) {
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr)

	destination, err := originalDestination(conn)
	if err != nil {
		logrus.Errorln("[NAT] get original destination error:", err)
		return
	}

	logrus.Debugf("[NAT] %s -> %s", remoteAddr.String(), destination.String())

	// Create proxy connection
	proxyConn, err := il.client.CreateProxy(ctx, destination)
	if err != nil {
		logrus.Errorln("[NAT] CreateProxy error:", err)
		return
	}
	defer proxyConn.Close()

	// Bidirectional forwarding
	_ = bufio.CopyConn(ctx, conn, proxyConn)
}
