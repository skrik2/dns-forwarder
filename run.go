package df

import (
	"flag"
	"log"

	"df/conf"
	"df/core"
	"df/server"
)

func Run() error {
	// define cmd
	configPath := flag.String("c", "", "config file")
	flag.Parse()

	// load config
	conf.LoadConfig(*configPath)
	log.Printf("[info] successfully loaded configuration file: %s\n", *configPath)
	log.Printf("%#v", conf.Info())

	// upstream
	core.Init()

	// Server
	_ = Server()
	return nil
}

func Server() error {
	cfg := conf.Info()

	go func() {
		if err := server.Standard(cfg.Server.Standard); err != nil {
			log.Printf("[error] standard: %v", err)
		}
	}()

	go func() {
		if err := server.Dot(); err != nil {
			log.Printf("[error] dot: %v", err)
		}
	}()

	go func() {
		if err := server.Doq(); err != nil {
			log.Printf("[error] doq: %v", err)
		}
	}()

	go func() {
		if err := server.Doh(); err != nil {
			log.Printf("[error] doh: %v", err)
		}
	}()
	select {}
}
