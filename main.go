package main

import (
	"net/http"
	"os"

	"github.com/hashicorp/go-hclog"
)

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       "pear",
		JSONFormat: true,
	})
	conf, err := NewConfig()
	if err != nil {
		logger.Error("failed loading config", "error", err)
		os.Exit(1)
	}
	logger.Info("config loaded")
	if conf.Debug {
		logger.SetLevel(hclog.Debug)
	}
	pear, err := NewPearService(conf, logger)
	if err != nil {
		logger.Error("failed to setup Pear service", "error", err)
		os.Exit(1)
	}
	r := NewRouter(pear)
	logger.Info("starting webserver on :3333")
	err = http.ListenAndServe(":"+conf.Port, r)
	if err != nil {
		logger.Error("http listener error", "error", err)
		os.Exit(1)
	}
}
