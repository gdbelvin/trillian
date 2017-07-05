// Copyright 2016 Google Inc. All Rights Reserved.
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

package merkle

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"sort"

	"github.com/google/trillian/merkle/hashers"
)

var (
	// ErrNegativeTreeLevelOffset indicates a negative level was specified.
	ErrNegativeTreeLevelOffset = errors.New("treeLevelOffset cannot be negative")
	smtOne                     = big.NewInt(1)
	smtZero                    = big.NewInt(0)
)

// HStar2LeafHash represents a leaf for the HStar2 sparse Merkle tree
// implementation.
type HStar2LeafHash struct {
	// TODO(al): remove big.Int
	Index    *big.Int
	LeafHash []byte
}

// HStar2 is a recursive implementation for calculating the root hash of a sparse
// Merkle tree.
type HStar2 struct {
	treeID int64
	hasher hashers.MapHasher
}

// NewHStar2 creates a new HStar2 tree calculator based on the passed in MapHasher.
func NewHStar2(treeID int64, hasher hashers.MapHasher) HStar2 {
	return HStar2{
		treeID: treeID,
		hasher: hasher,
	}
}

// HStar2Root calculates the root of a sparse Merkle tree of depth n which contains
// the given set of non-null leaves.
func (s *HStar2) HStar2Root(n int, values []HStar2LeafHash) ([]byte, error) {
	log.Printf("HStar2Root(%v, len values: %v)", n, len(values))
	sort.Sort(ByIndex{values})
	return s.hStar2b(n, values, smtZero,
		func(depth int, index *big.Int) ([]byte, error) {
			return s.hasher.HashEmpty(s.treeID, PaddedBytes(index, s.hasher.Size()), depth), nil
		},
		func(int, *big.Int, []byte) error { return nil })
}

// SparseGetNodeFunc should return any pre-existing node hash for the node address.
type SparseGetNodeFunc func(depth int, index *big.Int) ([]byte, error)

// SparseSetNodeFunc should store the passed node hash, associating it with the address.
type SparseSetNodeFunc func(depth int, index *big.Int, hash []byte) error

// HStar2Nodes calculates the root hash of a pre-existing sparse Merkle tree (SMT).
// HStar2Nodes can also calculate the root nodes of subtrees inside a SMT.
// Get and set are used to fetch and store internal node values.
// Values must not contain multiple leaves for the same index.
//
// index is the location of this subtree within the larger tree. Root is at nil.
// depth is the location of this subtree within the larger tree. Root is at 0.
// subtreeDepth is the number of levels in this subtree. Typically 8.
// The height of the whole tree is assumed to be hasher.BitLen()
func (s *HStar2) HStar2Nodes(index []byte, depth, subtreeDepth int, values []HStar2LeafHash,
	get SparseGetNodeFunc, set SparseSetNodeFunc) ([]byte, error) {
	treeDepth := subtreeDepth
	treeLevelOffset := s.hasher.BitLen() - depth - subtreeDepth
	log.Printf("HStar2Nodes(%v, %v, len values: %v)", treeDepth, treeLevelOffset, len(values))
	for _, v := range values {
		log.Printf("   v: %x : %x", v.Index.Bytes(), v.LeafHash)
	}
	if treeLevelOffset < 0 {
		return nil, ErrNegativeTreeLevelOffset
	}
	sort.Sort(ByIndex{values})
	offset := new(big.Int).SetBytes(index)
	return s.hStar2b(treeDepth, values, offset,
		func(depth int, index *big.Int) ([]byte, error) {
			// if we've got a function for getting existing node values, try it:
			h, err := get(treeDepth-depth, index)
			if err != nil {
				return nil, err
			}
			// if we got a value then we'll use that
			if h != nil {
				return h, nil
			}
			// otherwise just return the null hash for this level
			return s.hasher.HashEmpty(s.treeID, PaddedBytes(index, s.hasher.Size()), depth+treeLevelOffset), nil
		},
		func(depth int, index *big.Int, hash []byte) error {
			return set(treeDepth-depth, index, hash)
		})
}

// hStar2b computes a sparse Merkle tree root value recursively.
func (s *HStar2) hStar2b(n int, values []HStar2LeafHash, offset *big.Int, get SparseGetNodeFunc, set SparseSetNodeFunc) ([]byte, error) {
	if n == 0 {
		switch {
		case len(values) == 0:
			return get(n, offset)
		case len(values) != 1:
			return nil, fmt.Errorf("hStar2b base case: len(values): %d, want 1", len(values))
		}
		return values[0].LeafHash, nil
	}
	if len(values) == 0 {
		return get(n, offset)
	}

	split := new(big.Int).Lsh(smtOne, uint(n-1))
	split.Add(split, offset)
	i := sort.Search(len(values), func(i int) bool { return values[i].Index.Cmp(split) >= 0 })
	lhs, err := s.hStar2b(n-1, values[:i], offset, get, set)
	if err != nil {
		return nil, err
	}
	rhs, err := s.hStar2b(n-1, values[i:], split, get, set)
	if err != nil {
		return nil, err
	}
	h := s.hasher.HashChildren(lhs, rhs)
	if set != nil {
		set(n, offset, h)
	}
	return h, nil
}

// HStar2LeafHash sorting boilerplate below.

// Leaves is a slice of HStar2LeafHash
type Leaves []HStar2LeafHash

// Len returns the number of leaves.
func (s Leaves) Len() int { return len(s) }

// Swap swaps two leaf locations.
func (s Leaves) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// ByIndex implements sort.Interface by providing Less and using Len and Swap methods from the embedded Leaves value.
type ByIndex struct{ Leaves }

// Less returns true if i.Index < j.Index
func (s ByIndex) Less(i, j int) bool { return s.Leaves[i].Index.Cmp(s.Leaves[j].Index) < 0 }

// PaddedBytes takes a big.Int and returns it's value, left padded with zeros.
// e.g. 1 -> 0000000000000000000000000000000000000001
func PaddedBytes(i *big.Int, size int) []byte {
	b := i.Bytes()
	ret := make([]byte, size)
	padBytes := len(ret) - len(b)
	copy(ret[padBytes:], b)
	return ret
}
