package server

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"df/conf"
	"df/core"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
)

// RFC 9250
func Doq() error {
	cfg := conf.Info()
	cert, err := tls.LoadX509KeyPair(cfg.TLS.PublicKey, cfg.TLS.PrivateKey)
	if err != nil {
		log.Fatalf("[fatal] load cert/key: %v", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"doq"},
		MinVersion:   tls.VersionTLS13,
	}

	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.Doq)

	uc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen socket: %w", err)
	}

	idleTimeout := time.Duration(30) * time.Second
	quicConfig := &quic.Config{
		MaxIdleTimeout:                 idleTimeout,
		InitialStreamReceiveWindow:     4 * 1024,
		MaxStreamReceiveWindow:         4 * 1024,
		InitialConnectionReceiveWindow: 8 * 1024,
		MaxConnectionReceiveWindow:     16 * 1024,
		Allow0RTT:                      false,

		// UniStream is not allowed
		MaxIncomingUniStreams: -1,
	}

	srk, _, err := InitQUICSrkFromIfaceMac()
	if err != nil {
		log.Printf("[warn] can't create StatelessResetKey, it will be disable: %v", err)
	}

	quicTransport := &quic.Transport{
		Conn:              uc,
		StatelessResetKey: (*quic.StatelessResetKey)(srk),
	}

	listener, err := quicTransport.Listen(tlsCfg, quicConfig)
	if err != nil {
		log.Fatal("[fatal] can't start quic server")
	}
	defer listener.Close()

	log.Printf("[info] DoQ server started on %s", addr)

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("[error] accept connection: %v", err)
			continue
		}

		go handleQuicConn(conn)
	}

}

func handleQuicConn(conn *quic.Conn) {
	defer conn.CloseWithError(0, "")

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			log.Printf("[error] accept stream: %v", err)
			return
		}

		go handleQuicStream(stream)
	}
}

func handleQuicStream(stream *quic.Stream) {
	defer stream.Close()

	stream.SetReadDeadline(time.Now().Add(time.Second * 2))
	lengthBuf := make([]byte, 2)

	if _, err := io.ReadFull(stream, lengthBuf[:]); err != nil {
		log.Printf("[error] read message length: %v", err)
		return
	}
	msgLen := binary.BigEndian.Uint16(lengthBuf[:])

	if msgLen == 0 || msgLen > dns.MaxMsgSize {
		log.Printf("[error] invalid DNS message length: %d", msgLen)
		return
	}

	msgBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(stream, msgBuf); err != nil {
		log.Printf("[error] read DNS message: %v", err)
		return
	}

	req := new(dns.Msg)
	if err := req.Unpack(msgBuf); err != nil {
		log.Printf("[error] unpack DNS: %v", err)
		return
	}

	resp, err := core.Core(req)
	if err != nil {
		// 返回 SERVFAIL
		servfail := new(dns.Msg)
		servfail.SetRcode(req, dns.RcodeServerFailure)
		resp = servfail
	}

	respMsg, err := resp.Pack()
	if err != nil {
		log.Printf("[error] pack response: %v", err)
		return
	}

	out := make([]byte, 2+len(respMsg))
	binary.BigEndian.PutUint16(out[:2], uint16(len(respMsg)))
	copy(out[2:], respMsg)

	// 写回 QUIC stream
	_, err = stream.Write(out)
	if err != nil {
		log.Printf("[error] write stream: %v", err)
		return
	}
}
