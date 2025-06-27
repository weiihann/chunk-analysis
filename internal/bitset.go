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
	// setCount   uint32 // Number of accessed bytes
	// chunkCount uint32 // Number of 32-byte chunks that were accessed
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

	return b.set(index)
}

func (b *BitSet) SetWithCheck(index uint32) (*BitSet, error) {
	if index >= b.size {
		return nil, fmt.Errorf("index out of range (%d >= %d)", index, b.size)
	}

	return b.set(index), nil
}

func (b *BitSet) set(index uint32) *BitSet {
	wordIndex := index / 32
	bitIndex := index % 32

	mask := uint32(1 << bitIndex)
	// if b.bits[wordIndex]&mask == 0 {
	// 	b.setCount++
	// }

	// if b.bits[wordIndex] == 0 {
	// 	b.chunkCount++
	// }

	b.bits[wordIndex] |= mask

	return b
}

// Count the number of set bits in the BitSet
func (b *BitSet) Count() int {
	// if b.setCount != 0 {
	// 	return int(b.setCount)
	// }

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
	// if b.chunkCount != 0 {
	// 	return int(b.chunkCount)
	// }

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

func (b *BitSet) Merge(other *BitSet) *BitSet {
	if b.size != other.size {
		panic("size mismatch")
	}

	for i := range b.bits {
		b.bits[i] |= other.bits[i]
	}

	return b
}

func (b *BitSet) IsFull() bool {
	return b.Count() == int(b.size)
}
