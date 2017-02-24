// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/google/trillian"
	"github.com/google/trillian/ecosystem/logproxy"
	"github.com/google/trillian/monitoring"
	"github.com/google/trillian/testonly/loglb/balancer"
	"github.com/google/trillian/util"

	"google.golang.org/grpc"
)

var (
	backendsFlag     = flag.String("backends", "", "Comma-separated list of backends")
	serverPortFlag   = flag.Int("port", 8090, "Port to serve log RPC requests on")
	exportRPCMetrics = flag.Bool("export_metrics", true, "If true starts HTTP server and exports stats")
	httpPortFlag     = flag.Int("http_port", 8091, "Port to serve HTTP metrics on")
)

func startHTTPServer(port int) error {
	sock, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}
	go func() {
		glog.Info("HTTP server starting")
		http.Serve(sock, nil)
	}()
	return nil
}

func awaitSignal(rpcServer *grpc.Server) {
	// Arrange notification for the standard set of signals used to terminate a server
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Now block main and wait for a signal
	sig := <-sigs
	glog.Warningf("Signal received: %v", sig)
	glog.Flush()

	// Bring down the RPC server, which will unblock main
	rpcServer.Stop()
}

func main() {
	flag.Parse()
	glog.CopyStandardLogTo("WARNING")
	glog.Info("**** Log RPC Load Balancer Starting ****")

	// Start HTTP server (optional)
	if *exportRPCMetrics {
		if err := startHTTPServer(*httpPortFlag); err != nil {
			glog.Exitf("Failed to start http server on port %d: %v", *httpPortFlag, err)
		}
	}

	// Set up the listener for the server
	glog.Infof("Creating listener for port: %d", *serverPortFlag)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *serverPortFlag))
	if err != nil {
		glog.Errorf("Failed to listen on the server port: %d, because: %v", *serverPortFlag, err)
		os.Exit(1)
	}

	// Bring up the RPC server
	glog.Infof("Creating load balancer across %q", *backendsFlag)
	backendAddrs := strings.Split(*backendsFlag, ",")
	if len(backendAddrs) == 0 || (len(backendAddrs) == 1 && backendAddrs[0] == "") {
		glog.Fatalf("no backends specified")
	}
	b := balancer.New(backendAddrs)
	cc, err := grpc.Dial(backendAddrs[0], grpc.WithBalancer(b), grpc.WithInsecure(), grpc.WithBlock())
	log.Printf("Connected")
	if err != nil {
		glog.Fatalf("Could not connect: %v", err)
	}
	client := trillian.NewTrillianLogClient(cc)
	proxy := logproxy.New(client)

	statsInterceptor := monitoring.NewRPCStatsInterceptor(util.SystemTimeSource{}, "ct", "example")
	statsInterceptor.Publish()
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(statsInterceptor.Interceptor()))
	trillian.RegisterTrillianLogServer(grpcServer, proxy)

	glog.Infof("Creating RPC load balancing server on port: %d", *serverPortFlag)
	go awaitSignal(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		glog.Warningf("RPC server terminated on port %d: %v", *serverPortFlag, err)
		os.Exit(1)
	}

	// Give things a few seconds to tidy up
	glog.Infof("Stopping server, about to exit")
	glog.Flush()
	time.Sleep(time.Second * 5)
}
