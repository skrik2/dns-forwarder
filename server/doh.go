package server

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"df/conf"
	"df/core"

	"github.com/miekg/dns"
	"golang.org/x/net/http2"
)

func Doh() error {
	go func() {
		if err := Http2(); err != nil {
			fmt.Printf("UDP server error: %v\n", err)
		}
	}()
	select {}
}

// Http2()
//
// https://datatracker.ietf.org/doc/html/rfc8484
func Http2() error {
	cfg := conf.Info()
	path := cfg.Server.Doh.Path
	auths := cfg.Server.Doh.Auth

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		// Basic Auth
		auth := r.Header.Get("Authorization")
		isAuthed := dohBasicAuth(auth, auths)
		_ = isAuthed

		// Parse DNS Request
		var queryMsg []byte
		var err error
		switch r.Method {
		case http.MethodGet:
			dnsParam := r.URL.Query().Get("dns")
			if dnsParam == "" {
				http.Error(w, "missing dns param", http.StatusBadRequest)
				return
			}
			queryMsg, err = base64.RawURLEncoding.DecodeString(dnsParam)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid dns param: %v", err), http.StatusBadRequest)
				return
			}
		case http.MethodPost:
			if r.Header.Get("Content-Type") != "application/dns-message" {
				http.Error(w, "invalid content-type", http.StatusUnsupportedMediaType)
				return
			}
			queryMsg, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		req := new(dns.Msg)
		if err := req.Unpack(queryMsg); err != nil {
			http.Error(w, fmt.Sprintf("failed to parse DNS message: %v", err), http.StatusBadRequest)
			return
		}

		// DNS Exchange
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

		w.Header().Set("Content-Type", "application/dns-message")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		w.Write(respMsg)
	})

	// HTTP/2
	addr := fmt.Sprintf(":%d", cfg.Server.Doh.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        mux,
		IdleTimeout:    time.Duration(30) * time.Second,
		MaxHeaderBytes: 512,
		TLSConfig: &tls.Config{
			NextProtos: []string{"h2"}, // 禁用 HTTP/1
		},
	}
	if err := http2.ConfigureServer(srv, &http2.Server{
		MaxReadFrameSize:             16 * 1024,
		IdleTimeout:                  time.Duration(30) * time.Second,
		MaxUploadBufferPerStream:     65535,
		MaxUploadBufferPerConnection: 65535,
	}); err != nil {
		return fmt.Errorf("failed to configure http2: %w", err)
	}

	log.Printf("[info] DoH server (HTTP/2) started on: %s", addr)
	err := srv.ListenAndServeTLS(cfg.TLS.PublicKey, cfg.TLS.PrivateKey)
	if err != nil {
		return fmt.Errorf("[fatal] can't start http/2 server")
	}
	return nil
}

// Basic Auth
func dohBasicAuth(auth string, auths []conf.AuthDohItem) bool {
	if auth == "" {
		return false
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return false
	}

	payload, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	userPass := strings.SplitN(string(payload), ":", 2)
	if len(userPass) != 2 {
		return false
	}

	for _, v := range auths {
		if v.User == userPass[0] && v.Password == userPass[1] {
			return true
		}
	}

	return false
}

func Http3() error {
	return nil
}
