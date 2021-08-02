package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/sarchlab/akkalat/runner"
	"gitlab.com/akita/mgpusim/v2/server"
)

func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	server.MakeBuilder().WithDriver(runner.Driver()).Build()
	server.RegisterHandlers()
	log.Fatal(http.ListenAndServe(":8081", nil))
}
