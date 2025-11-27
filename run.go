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
	//TDDO: Panel

	// Standard
	_ = server.Standard(cfg.Server.Standard)
	return nil
}
