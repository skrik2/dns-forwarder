package server

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	// internal
	"df/conf"
	"df/core"

	"github.com/miekg/dns"
)

func Dot() error {
	cfg := conf.Info()
	cert, err := tls.LoadX509KeyPair(cfg.TLS.PublicKey, cfg.TLS.PrivateKey)
	if err != nil {
		log.Fatalf("[fatal] load cert/key: %v", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.Dot)
	listener, err := tls.Listen("tcp", addr, tlsCfg)
	if err != nil {
		log.Fatalf("[fatal] can't create dot server: %v", err)
	}
	defer listener.Close()
	log.Printf("[info] DOT server started on: %s", addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}

		go handleDOTConn(conn)
	}
}

func handleDOTConn(conn net.Conn) {
	defer conn.Close()
	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		lengthBuf := make([]byte, 2)
		_, err := io.ReadFull(conn, lengthBuf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("[info] tcp client closed connection")
				return
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				log.Printf("[warn] tcp read timeout: %v", err)
				return
			}
			return
		}

		msgLen := int(binary.BigEndian.Uint16(lengthBuf))
		if msgLen == 0 || msgLen > 64*1024 {
			log.Printf("[error] invalid dns length=%d", msgLen)
			return
		}

		msgBuf := make([]byte, msgLen)
		_, err = io.ReadFull(conn, msgBuf)
		if err != nil {
			log.Printf("[error] tcp read dns msg: %v", err)
			return
		}

		req := new(dns.Msg)
		if err := req.Unpack(msgBuf); err != nil {
			log.Printf("[error] bad dns packet: %v", err)
			return
		}

		resp, err := core.Core(req)
		if err != nil {
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

		_, err = conn.Write(out)
		if err != nil {
			log.Printf("[error] write tcp response: %v", err)
			return
		}

		// 刷新 deadline，防止长连接挂住
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	}
}
