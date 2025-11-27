package server

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"df/core"

	"github.com/miekg/dns"
)

func Standard(port int) error {
	go func() {
		if err := Udp(port); err != nil {
			fmt.Printf("UDP server error: %v\n", err)
		}
	}()

	go func() {
		if err := Tcp(port); err != nil {
			fmt.Printf("TCP server error: %v\n", err)
		}
	}()
	select {}
}

func Udp(port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address %s: %w", addr, err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to bind UDP address %s: %w", addr, err)
	}
	defer udpConn.Close()

	log.Printf("[info] Standard Server (TCP) started on: %s", addr)

	buf := make([]byte, dns.MinMsgSize)
	for {
		n, clientAddr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read error: %v\n", err)
			continue
		}

		req := new(dns.Msg)
		if err := req.Unpack(buf[:n]); err != nil {
			log.Printf("[error] bad dns packet: %v", err)
			continue
		}

		resp, err := core.Core(req)
		if err != nil {
			log.Printf("Core error: %v\n", err)
			continue
		}

		maxSize := dns.MinMsgSize
		if o := req.IsEdns0(); o != nil {
			if o.UDPSize() > 512 {
				maxSize = int(o.UDPSize())
			}
		}
		resp.Truncate(maxSize)

		msg, err := resp.Pack()
		if err != nil {
			log.Printf("[error] bad dns response: %v", err)
		}
		log.Printf("[info] query dns %v", resp.Answer)
		_, err = udpConn.WriteToUDP(msg, clientAddr)
		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
}

func Tcp(port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address %s: %w", addr, err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("failed to bind TCP address %s: %w", addr, err)
	}
	defer listener.Close()

	log.Printf("[info] Standard Server (TCP) started on: %s", addr)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Printf("[error] tcp accept error: %v", err)
		}
		go handleTCPConn(conn)
	}

}

func handleTCPConn(conn *net.TCPConn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	for {
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
			log.Printf("[error] core: %v", err)
			return
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
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	}
}
