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
)

func TestNewNodeIDWithPrefix(t *testing.T) {
	for _, tc := range []struct {
		input     uint64
		inputLen  int
		prefixLen int
		indexLen  int
		want      []byte
	}{
		{
			input:     0,
			inputLen:  0,
			prefixLen: 0,
			indexLen:  64,
			want:      h2b("0000000000000000"),
		},
		{
			input:     0x12345678,
			inputLen:  32,
			prefixLen: 32,
			indexLen:  64,
			want:      h2b("1234567800000000"),
		},
		{
			input:     0x345678,
			inputLen:  15,
			prefixLen: 15,
			indexLen:  24,
			want:      h2b("acf000"), // bottom 15 bits of 0x345678 are: 1010 1100 1111 000x
		},
	} {
		n := NewNodeIDWithPrefix(tc.input, tc.inputLen, tc.prefixLen, tc.indexLen)
		if got, want := n.Path, tc.want; !bytes.Equal(got, want) {
			t.Errorf("NewNodeIDWithPrefix(%x, %v, %v, %v).Path: %x, want %x", got, want)
		}
	}

}

var nodeIDForTreeCoordsVec = []struct {
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
}

func TestNewNodeIDForTreeCoords(t *testing.T) {
	for i, v := range nodeIDForTreeCoordsVec {
		n, err := NewNodeIDForTreeCoords(v.depth, v.index, v.maxBits)

		switch {
		case err != nil && v.shouldFail:
			// pass
			continue
		case err == nil && !v.shouldFail:
			if got, want := n.String(), v.expected; got != want {
				t.Errorf("(test vector index %d) Expected '%s', got '%s', %v", i, want, got, err)
			}
		case err != nil && v.shouldFail:
			t.Errorf("unexpectedly created a node ID for test vector entry %d, should've failed because %s", i, v.expected)
			continue
		case err == nil && !v.shouldFail:
			t.Errorf("failed to create nodeID for test vector entry %d: %v", i, err)
			continue
		}

	}
}

func TestSetBit(t *testing.T) {
	n := NewNodeIDWithPrefix(0, 0, 0, 64)
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
	n := NewNodeIDWithPrefix(0x9249, 16, 16, 16)
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
	for _, tc := range []struct {
		n    NodeID
		want string
	}{
		{
			n:    NewEmptyNodeID(32),
			want: "",
		},
		{
			n:    NewNodeIDWithPrefix(0x345678, 24, 32, 32),
			want: "00110100010101100111100000000000",
		},
		{
			n:    NewNodeIDWithPrefix(0x12345678, 32, 32, 64),
			want: "00010010001101000101011001111000",
		},
		{
			n:    NewNodeIDWithPrefix(0x345678, 15, 15, 24),
			want: fmt.Sprintf("%015b", 0x345678&0x7fff),
		},
	} {
		if got, want := tc.n.String(), tc.want; got != want {
			t.Errorf("%v: String():  %v,  want %v'", got, want)
		}
	}
}

func TestSiblings(t *testing.T) {
	for _, tc := range []struct {
		input     uint64
		inputLen  int
		prefixLen int
		indexLen  int
		want      []string
	}{
		{
			input:     0xabe4,
			inputLen:  16,
			prefixLen: 16,
			indexLen:  16,
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
		n := NewNodeIDWithPrefix(tc.input, tc.inputLen, tc.prefixLen, tc.indexLen)
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
	na := NewNodeIDWithPrefix(0x1234, l, l, l)
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
			n1:   NewNodeIDWithPrefix(0x1234, l, l, l),
			n2:   NewNodeIDWithPrefix(0x1234, l, l, l),
			want: true,
		},
		{
			// Different PrefixLen
			n1:   NewNodeIDWithPrefix(0x1234, l, l, l),
			n2:   NewNodeIDWithPrefix(0x1234, l-1, l, l),
			want: false,
		},
		{
			// Different IDLen
			n1:   NewNodeIDWithPrefix(0x1234, l, l, l),
			n2:   NewNodeIDWithPrefix(0x1234, l, l+1, l+1),
			want: false,
		},
		{
			// Different Prefix
			n1:   NewNodeIDWithPrefix(0x1234, l, l, l),
			n2:   NewNodeIDWithPrefix(0x5432, l, l, l),
			want: false,
		},
		{
			// Different max len, but that's ok because the prefixes are identical
			n1:   NewNodeIDWithPrefix(0x1234, l, l, l),
			n2:   NewNodeIDWithPrefix(0x1234, l, l, l*2),
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
