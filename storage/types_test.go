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

package storage

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/google/trillian/node"
)

func TestNewNodeIDWithPrefix(t *testing.T) {
	for _, tc := range []struct {
		input    []byte
		inputLen int
		pathLen  int
		maxLen   int
		want     []byte
	}{
		{
			input:    h2b(""),
			inputLen: 0,
			pathLen:  0,
			maxLen:   64,
			want:     h2b("0000000000000000"),
		},
		{
			input:    h2b("12345678"),
			inputLen: 32,
			pathLen:  32,
			maxLen:   64,
			want:     h2b("1234567800000000"),
		},
		{
			input:    h2b("345678"),
			inputLen: 15,
			pathLen:  15,
			maxLen:   24,
			want:     h2b("345600"), // top 15 bits of 0x345678 are: 0101 0110 0111 1000
		},
	} {
		n := NewNodeIDWithPrefix(tc.input, tc.inputLen, tc.pathLen, tc.maxLen)
		if got, want := n.Path, tc.want; !bytes.Equal(got, want) {
			t.Errorf("NewNodeIDWithPrefix(%x, %v, %v, %v).Path: %x, want %x",
				tc.input, tc.inputLen, tc.pathLen, tc.maxLen, got, want)
		}
	}

}

func TestNewNodeIDForTreeCoords(t *testing.T) {
	for i, v := range []struct {
		depth      int64
		index      int64
		maxBits    int
		shouldFail bool
		expected   string
	}{
		{0, 0x00, 8, false, "00000000"},
		{0, 0x01, 8, false, "00000001"},
		{0, 0x01, 15, false, "000000000000001"},
		{1, 0x01, 8, false, "0000001"},
		{2, 0x04, 8, false, "000100"},
		{8, 0x01, 16, false, "00000001"},
		{8, 0x01, 9, false, "1"},
		{0, 0x80, 8, false, "10000000"},
		{0, 0x01, 64, false, "0000000000000000000000000000000000000000000000000000000000000001"},
		{63, 0x01, 64, false, "1"},
		{63, 0x02, 64, true, "index of 0x02 is too large for given depth"},
	} {
		n, err := NewNodeIDForTreeCoords(v.depth, v.index, v.maxBits)

		if got, want := err != nil, v.shouldFail; got != want {
			t.Errorf("%v: NewNodeIDForTreeCoords(): %v, want failure: %v", i, err, want)
			continue
		}
		if err != nil {
			continue
		}
		if got, want := n.String(), v.expected; got != want {
			t.Errorf("(test vector index %d) Expected '%s', got '%s', %v", i, want, got, err)
		}
	}
}

func TestSetBit(t *testing.T) {
	n := NewNodeIDWithPrefix(h2b(""), 0, 0, 64)
	n.SetBit(27, 1)
	if got, want := n.Path, []byte{0x00, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00}; !bytes.Equal(got, want) {
		t.Fatalf("Expected Path of %v, but got %v", want, got)
	}

	n.SetBit(27, 0)
	if got, want := n.Path, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}; !bytes.Equal(got, want) {
		t.Fatalf("Expected Path of %v, but got %v", want, got)
	}
}

func TestBit(t *testing.T) {
	// every 3rd bit set
	n := NewNodeIDWithPrefix(h2b("9249"), 16, 16, 16)
	for x := 0; x < 16; x++ {
		want := 0
		if x%3 == 0 {
			want = 1
		}
		if got := n.Bit(x); got != uint(want) {
			t.Fatalf("Expected bit %d to be %d, but got %d", x, want, got)
		}
	}
}

func TestString(t *testing.T) {
	for i, tc := range []struct {
		n    NodeID
		want string
	}{
		{
			n:    NewEmptyNodeID(32),
			want: "",
		},
		{
			n:    NewNodeIDWithPrefix(h2b("345678"), 24, 32, 32),
			want: "00110100010101100111100000000000",
		},
		{
			n:    NewNodeIDWithPrefix(h2b("12345678"), 32, 32, 64),
			want: "00010010001101000101011001111000",
		},
		{
			n:    NewNodeIDWithPrefix(h2b("345678"), 15, 15, 24),
			want: fmt.Sprintf("%015b", 0x3456),
		},
	} {
		if got, want := tc.n.String(), tc.want; got != want {
			t.Errorf("%v: String():  %v,  want '%v'", i, got, want)
		}
	}
}

func TestSiblings(t *testing.T) {
	for _, tc := range []struct {
		input    []byte
		inputLen int
		pathLen  int
		maxLen   int
		want     []string
	}{
		{
			input:    h2b("abe4"),
			inputLen: 16,
			pathLen:  16,
			maxLen:   16,
			want: []string{"1010101111100101",
				"101010111110011",
				"10101011111000",
				"1010101111101",
				"101010111111",
				"10101011110",
				"1010101110",
				"101010110",
				"10101010",
				"1010100",
				"101011",
				"10100",
				"1011",
				"100",
				"11",
				"0"},
		},
	} {
		n := NewNodeIDWithPrefix(tc.input, tc.inputLen, tc.pathLen, tc.maxLen)
		sibs := n.Siblings()
		if got, want := len(sibs), len(tc.want); got != want {
			t.Errorf("Got %d siblings, want %d", got, want)
			continue
		}

		for i, s := range sibs {
			if got, want := s.String(), tc.want[i]; got != want {
				t.Errorf("sibling %d: %v, want %v", i, got, want)
			}
		}
	}
}

func TestNodeEquivalent(t *testing.T) {
	l := 16
	na := NewNodeIDWithPrefix(h2b("1234"), l, l, l)
	for _, tc := range []struct {
		n1, n2 NodeID
		want   bool
	}{
		{
			// Self is Equal
			n1:   na,
			n2:   na,
			want: true,
		},
		{
			// Equal
			n1:   NewNodeIDWithPrefix(h2b("1234"), l, l, l),
			n2:   NewNodeIDWithPrefix(h2b("1234"), l, l, l),
			want: true,
		},
		{
			// Different PrefixLen
			n1:   NewNodeIDWithPrefix(h2b("1234"), l, l, l),
			n2:   NewNodeIDWithPrefix(h2b("1234"), l-1, l, l),
			want: false,
		},
		{
			// Different IDLen
			n1:   NewNodeIDWithPrefix(h2b("1234"), l, l, l),
			n2:   NewNodeIDWithPrefix(h2b("1234"), l, l+1, l+1),
			want: false,
		},
		{
			// Different Prefix
			n1:   NewNodeIDWithPrefix(h2b("1234"), l, l, l),
			n2:   NewNodeIDWithPrefix(h2b("5432"), l, l, l),
			want: false,
		},
		{
			// Different max len, but that's ok because the prefixes are identical
			n1:   NewNodeIDWithPrefix(h2b("1234"), l, l, l),
			n2:   NewNodeIDWithPrefix(h2b("1234"), l, l, l*2),
			want: true,
		},
	} {
		if got, want := tc.n1.Equivalent(tc.n2), tc.want; got != want {
			t.Errorf("Equivalent(%v, %v): %v, want %v", tc.n1, tc.n2, got, want)
		}
	}
}

// It's important to have confidence in the CoordString output as it's used in debugging
func TestCoordString(t *testing.T) {
	// Test some roundtrips for various depths and indices
	for d := 0; d < 37; d++ {
		for i := 0; i < 117; i++ {
			n, err := NewNodeIDForTreeCoords(int64(d), int64(i), 64)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := n.CoordString(), fmt.Sprintf("[d:%d, i:%d]", d, i); got != want {
				t.Errorf("n.CoordString() got: %v, want: %v", got, want)
			}
		}
	}
}

// h2b converts a hex string into []byte.
func h2b(h string) []byte {
	b, err := hex.DecodeString(h)
	if err != nil {
		panic("invalid hex string")
	}
	return b
}

// NewNodeIDWithPrefix creates a new NodeID of nodeIDLen bits with the prefixLen MSBs set to prefix.
func NewNodeIDWithPrefix(prefix []byte, prefixBits, pathBits, maxBits int) NodeID {
	path := make([]byte, bytesForBits(maxBits))

	// Copy prefixBits of prefix into path.
	pfx := node.Prefix(prefix, len(prefix)*8, prefixBits)
	copy(path, pfx)

	return NodeID{
		Path:          path,
		PrefixLenBits: prefixBits,
		PathLenBits:   pathBits,
	}

}
