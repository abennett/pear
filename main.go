package main

import (
	"net/http"

	"github.com/hashicorp/go-hclog"
)

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "pear",
		Level: hclog.Debug,
	})
	conf, err := NewConfig()
	if err != nil {
		panic(err)
	}
	logger.Info("config loaded")
	pear := NewPearService(conf, logger)
	r := NewRouter(pear)
	logger.Info("starting webserver on :3333")
	http.ListenAndServe(":"+conf.Port, r)
}
