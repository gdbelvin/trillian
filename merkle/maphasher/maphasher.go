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

// Package maphasher provides hashing for maps.
package maphasher

import (
	"crypto"
	"fmt"
	"log"

	"github.com/google/trillian"
	"github.com/google/trillian/merkle/hashers"
)

func init() {
	hashers.RegisterMapHasher(trillian.HashStrategy_TEST_MAP_HASHER, Default)
}

// Domain separation prefixes
const (
	leafHashPrefix = 0
	nodeHashPrefix = 1
)

// Default is a SHA256 based MapHasher for maps.
var Default = New(crypto.SHA256)

// MapHasher implements a sparse merkel tree hashing algorithm. For testing only.
// It matches the test vectors generated by other sparse map implememtations,
// but it does not offer the full N bit security of the underlying hash function.
type MapHasher struct {
	crypto.Hash
	nullHashes [][]byte
}

// New creates a new merkel.MapHasher using the passed in hash function.
func New(h crypto.Hash) hashers.MapHasher {
	m := &MapHasher{Hash: h}
	m.initNullHashes()
	return m
}

// String returns a string representation for debugging.
func (m *MapHasher) String() string {
	return fmt.Sprintf("MapHasher{%v}", m.Hash)
}

// HashEmpty returns the hash of an empty branch at a given depth.
// A depth of 0 indicates the hash of an empty leaf.
// Empty branches within the tree are plain interior nodes e1 = H(e0, e0) etc.
func (m *MapHasher) HashEmpty(treeID int64, index []byte, height int) []byte {
	if height < 0 || height >= len(m.nullHashes) {
		panic(fmt.Sprintf("HashEmpty(%v) out of bounds", height))
	}
	depth := m.BitLen() - height
	log.Printf("HashEmpty(%x, %d): %x", index, depth, m.nullHashes[height])
	return m.nullHashes[height]
}

// HashLeaf returns the Merkle tree leaf hash of the data passed in through leaf.
// The hashed structure is leafHashPrefix||leaf.
func (m *MapHasher) HashLeaf(treeID int64, index []byte, height int, leaf []byte) []byte {
	h := m.New()
	h.Write([]byte{leafHashPrefix})
	h.Write(leaf)
	r := h.Sum(nil)
	depth := m.BitLen() - height
	log.Printf("HashEmpty(%x, %d): %x", index, depth, r)
	return r
}

// HashChildren returns the internal Merkle tree node hash of the the two child nodes l and r.
// The hashed structure is NodeHashPrefix||l||r.
func (m *MapHasher) HashChildren(l, r []byte) []byte {
	h := m.New()
	h.Write([]byte{nodeHashPrefix})
	h.Write(l)
	h.Write(r)
	p := h.Sum(nil)
	log.Printf("HashChildren(%x, %x): %x", l, r, p)
	return p
}

// BitLen returns the number of bits in the hash function.
func (m *MapHasher) BitLen() int {
	return m.Size() * 8
}

// initNullHashes sets the cache of empty hashes, one for each level in the sparse tree,
// starting with the hash of an empty leaf, all the way up to the root hash of an empty tree.
// These empty branches are not stored on disk in a sparse tree. They are computed since their
// values are well-known.
func (m *MapHasher) initNullHashes() {
	// Leaves are stored at depth 0. Root is at Size()*8.
	// There are Size()*8 edges, and Size()*8 + 1 nodes in the tree.
	nodes := m.Size()*8 + 1
	r := make([][]byte, nodes, nodes)
	r[0] = m.HashLeaf(0, nil, m.Size()*8, nil)
	for i := 1; i < nodes; i++ {
		r[i] = m.HashChildren(r[i-1], r[i-1])
	}
	m.nullHashes = r
}
