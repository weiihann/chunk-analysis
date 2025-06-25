package internal

import (
	"fmt"
	"math/bits"
)

const maxContractBytes = 24576

// Each bit represents a byte in the contract code.
// Only represent up to 24,576 bytes because that's the current max contract size.
// It is in big endian order. Least significant bit is the first byte.
type BitSet struct {
	bits []uint32
	size uint32 // Contract size in bytes
}

func NewBitSet(size uint32) *BitSet {
	if size == 0 {
		panic("size must be greater than 0")
	}

	if size > maxContractBytes {
		panic(fmt.Sprintf("size out of range (%d > max contract size)", size))
	}

	return &BitSet{
		bits: make([]uint32, (size+31)/32),
		size: size,
	}
}

func (b *BitSet) Set(index uint32) *BitSet {
	if index >= b.size {
		panic(fmt.Sprintf("index out of range (%d >= %d)", index, b.size))
	}

	wordIndex := index / 32
	bitIndex := index % 32
	b.bits[wordIndex] |= 1 << bitIndex

	return b
}

// Count the number of set bits in the BitSet
func (b *BitSet) Count() int {
	count := 0
	for _, word := range b.bits {
		count += bits.OnesCount32(word)
	}
	return count
}

// Get the proportion of the contract that is accessed.
func (b *BitSet) Proportion() float64 {
	return float64(b.Count()) / float64(b.size)
}

// Count the number of chunks that were at least accessed once.
func (b *BitSet) ChunkCount() int {
	count := 0
	for _, word := range b.bits {
		if word != 0 {
			count++
		}
	}
	return count
}

// Get the proportion of the contract that was accessed.
func (b *BitSet) ChunkProportion() float64 {
	return float64(b.ChunkCount()) / float64(len(b.bits))
}
