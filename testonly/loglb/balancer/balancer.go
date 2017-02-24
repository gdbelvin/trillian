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

// Package balancer contains a random load balancer.
package balancer

import (
	"errors"
	"log"

	context "golang.org/x/net/context"

	"google.golang.org/grpc"
)

// Random implements grpc.Balancer
type Random struct {
	connectedAddrs map[grpc.Address]bool
	notify         chan []grpc.Address
}

// New returns a random grpc load balancer.
func New(addresses []string) *Random {
	addrs := make([]grpc.Address, 0, len(addresses))
	for _, a := range addresses {
		addrs = append(addrs, grpc.Address{Addr: a})
	}

	r := &Random{
		notify: make(chan []grpc.Address, 1),
	}
	// Tell grpc to connect to the given addresses.
	log.Printf("Instructing grpc to connect to: %v", addrs)
	r.notify <- addrs
	return r
}

// Start collects initial data for load balancing.
func (r *Random) Start(target string, config grpc.BalancerConfig) error {
	log.Printf("Random balancer starting with: %v", target)
	return nil
}

// Up adds a connected address to the pool.
func (r *Random) Up(addr grpc.Address) (down func(error)) {
	log.Printf("Connected to: %v", addr)
	r.connectedAddrs[addr] = true
	return func(e error) {
		log.Printf("Disconnected from: %v", addr)
		delete(r.connectedAddrs, addr)
	}
}

// Get returns a random address from the connected address pool.
func (r *Random) Get(ctx context.Context, opts grpc.BalancerGetOptions) (addr grpc.Address, put func(), err error) {
	// Go randomizes iterating over maps.
	for addr := range r.connectedAddrs {
		log.Printf("Returning random address: %v", addr)
		return addr, func() {}, nil
	}
	return grpc.Address{}, func() {}, errors.New("No connected addresses")
}

// Notify returns a chanel of addresses that grpc internals will connect to.
func (r *Random) Notify() <-chan []grpc.Address {
	return r.notify
}

// Close shuts down this load balancer
func (r *Random) Close() error {
	// Disconnect from all backends.
	r.notify <- []grpc.Address{}
	return nil
}
