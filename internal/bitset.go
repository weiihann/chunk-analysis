package internal

import (
	"encoding/base64"
	"fmt"
	"math/bits"
)

const (
	maxContractBytes = 24576
	chunkSize        = 31
)

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
		bits: make([]uint32, (size+chunkSize-1)/chunkSize),
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
	wordIndex := index / chunkSize
	bitIndex := index % chunkSize

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

// Return a slice of bytes where each byte is the number of bytes accessed in the corresponding chunk.
func (b *BitSet) Chunks() []byte {
	chunks := make([]byte, len(b.bits))
	for i, word := range b.bits {
		if word != 0 {
			chunks[i] = byte(bits.OnesCount32(word))
		}
	}

	return chunks
}

func (b *BitSet) EncodeChunks() string {
	chunks := b.Chunks()
	encoded := base64.StdEncoding.EncodeToString(chunks)
	return encoded
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

func (b *BitSet) Size() uint32 {
	return b.size
}

// ChunkEfficiencyStats represents statistics about chunk usage efficiency
type ChunkEfficiencyStats struct {
	TotalChunks       int                // Total number of chunks in the contract
	AccessedChunks    int                // Number of chunks with at least one byte accessed (same as ChunkCount)
	AverageEfficiency float64            // Average efficiency of accessed chunks (0-1)
	Distribution      [chunkSize + 1]int // Distribution of chunks by bytes accessed (index 0 unused, 1-32 used)
}

// GetChunkEfficiencyStats analyzes how efficiently each 32-byte chunk is used
func (b *BitSet) GetChunkEfficiencyStats() ChunkEfficiencyStats {
	stats := ChunkEfficiencyStats{
		TotalChunks:    len(b.bits),
		AccessedChunks: 0,
	}

	totalBytesInAccessedChunks := 0

	for _, word := range b.bits {
		if word != 0 {
			// This chunk has at least one byte accessed
			stats.AccessedChunks++

			// Count how many bytes are accessed in this chunk
			bytesAccessed := bits.OnesCount32(word)
			totalBytesInAccessedChunks += bytesAccessed

			// Update distribution (index 0 is unused, 1-32 are used)
			stats.Distribution[bytesAccessed]++
		}
	}

	// Calculate average efficiency
	if stats.AccessedChunks > 0 {
		stats.AverageEfficiency = float64(totalBytesInAccessedChunks) / float64(stats.AccessedChunks*chunkSize)
	}

	return stats
}

// GetChunkEfficiencies returns the efficiency (bytes accessed / 32) for each chunk
// Only includes chunks that have at least one byte accessed
func (b *BitSet) GetChunkEfficiencies() []float64 {
	var efficiencies []float64

	for _, word := range b.bits {
		if word != 0 {
			bytesAccessed := bits.OnesCount32(word)
			efficiency := float64(bytesAccessed) / float64(chunkSize)
			efficiencies = append(efficiencies, efficiency)
		}
	}

	return efficiencies
}

// GetChunkDetails returns detailed information about each chunk
// Returns a slice where each element represents a chunk with its index and bytes accessed
type ChunkDetail struct {
	Index         int     // Chunk index (0-based)
	BytesAccessed int     // Number of bytes accessed in this chunk (0-32)
	Efficiency    float64 // Efficiency of this chunk (0-1)
}

func (b *BitSet) GetChunkDetails() []ChunkDetail {
	var details []ChunkDetail

	for i, word := range b.bits {
		if word != 0 {
			bytesAccessed := bits.OnesCount32(word)
			details = append(details, ChunkDetail{
				Index:         i,
				BytesAccessed: bytesAccessed,
				Efficiency:    float64(bytesAccessed) / float64(chunkSize),
			})
		}
	}

	return details
}
