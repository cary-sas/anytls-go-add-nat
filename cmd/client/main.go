package main

import (
	"anytls/proxy"
	"anytls/util"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"flag"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var passwordSha256 []byte

func main() {
	// Original parameters
	serverAddr := flag.String("s", "", "Server address or anytls:// link")
	sni := flag.String("sni", "", "Server Name Indication")
	password := flag.String("p", "", "Password")
	minIdleSession := flag.Int("m", 5, "Reserved min idle session")
	listenAddr := flag.String("l", "", "SOCKS5 listen address, compatibility alias for -socks")

	// New parameters for dual-mode support
	socksAddr := flag.String("socks", "", "SOCKS5 listen address (e.g., 127.0.0.1:1080)")
	natAddr := flag.String("nat", "", "NAT listen address (e.g., 0.0.0.0:3333)")

	flag.Parse()

	if serverURL, err := url.Parse(*serverAddr); err == nil {
		if serverURL.Scheme == "anytls" {
			*serverAddr = serverURL.Host
			if serverURL.User != nil {
				*password = serverURL.User.String()
			}
			query := serverURL.Query()
			*sni = query.Get("sni")
		}
	}

	if *serverAddr == "" {
		logrus.Fatalln("please set -s server address")
	}

	if *password == "" {
		logrus.Fatalln("please set -p password")
	}

	if _, _, err := net.SplitHostPort(*serverAddr); err != nil {
		logrus.Fatalln("error server address:", *serverAddr, err)
	}

	if *socksAddr == "" && *listenAddr != "" {
		*socksAddr = *listenAddr
	}

	logLevel, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)

	var sum = sha256.Sum256([]byte(*password))
	passwordSha256 = sum[:]

	logrus.Infoln("[Client]", util.ProgramVersionName)

	// You can only use `InsecureSkipVerify` by default in the sample client; it is not recommended for use in production code.
	tlsConfig := &tls.Config{
		ServerName:         *sni,
		InsecureSkipVerify: true,
	}
	if tlsConfig.ServerName == "" {
		// disable the SNI
		tlsConfig.ServerName = "127.0.0.1"
	}

	path := strings.TrimSpace(os.Getenv("TLS_KEY_LOG"))
	if path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err == nil {
			tlsConfig.KeyLogWriter = f
		}
	}

	ctx := context.Background()

	// Create anytls client
	client := NewMyClient(ctx, func(ctx context.Context) (net.Conn, error) {
		conn, err := proxy.SystemDialer.DialContext(ctx, "tcp", *serverAddr)
		if err != nil {
			return nil, err
		}
		conn = tls.Client(conn, tlsConfig)
		return conn, nil
	}, *minIdleSession)

	// Create inbound listeners
	var inbounds []*InboundListener

	// SOCKS5 inbound
	if *socksAddr != "" {
		logrus.Infoln("[SOCKS5] Listening on", *socksAddr)
		socksListener := NewInboundListener(*socksAddr, "socks5", client)
		inbounds = append(inbounds, socksListener)
		go socksListener.Start(ctx)
	}

	// NAT inbound
	if *natAddr != "" {
		logrus.Infoln("[NAT] Listening on", *natAddr)
		natListener := NewInboundListener(*natAddr, "nat", client)
		inbounds = append(inbounds, natListener)
		go natListener.Start(ctx)
	}

	if len(inbounds) == 0 {
		logrus.Fatalln("please set at least -socks or -nat")
	}

	logrus.Infoln("[Client] Server:", *serverAddr)
	logrus.Infoln("[Client] Started successfully!")

	// Keep running
	select {}
}
