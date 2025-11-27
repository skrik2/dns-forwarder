package server

import (
	"fmt"
	"net/http"
	"strconv"
)

func Doh(port int) error {
	//
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed, must be POST", http.StatusMethodNotAllowed)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/dns-message" {
			http.Error(w, "Unsupported Content-Type, must be application/dns-message", http.StatusUnsupportedMediaType)
			return
		}

		w.Header().Set("Content-Type", "application/dns-message")
		fmt.Fprintln(w, "Welcome to the homepage!")
	})

	//  /dns-query
	http.HandleFunc("/dns-query", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "This is the DNS query endpoint.")
	})

	//  /statistics.html
	http.HandleFunc("/statistics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Request statistics page.")
	})

	// Router
	fmt.Println("DOH Server is running at http://localhost:", port)
	if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
		fmt.Println("Server failed:", err)
	}
	return nil
}
