package internal

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestChunkEfficiencyStats(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *BitSet
		expected ChunkEfficiencyStats
	}{
		{
			name: "Empty bitset",
			setup: func() *BitSet {
				return NewBitSet(128) // 4 chunks
			},
			expected: ChunkEfficiencyStats{
				TotalChunks:       4,
				AccessedChunks:    0,
				AverageEfficiency: 0,
				Distribution:      [32]int{},
			},
		},
		{
			name: "Single byte in first chunk",
			setup: func() *BitSet {
				b := NewBitSet(128)
				b.Set(0)
				return b
			},
			expected: ChunkEfficiencyStats{
				TotalChunks:       4,
				AccessedChunks:    1,
				AverageEfficiency: 1.0 / 32.0,
				Distribution:      func() [32]int { var d [32]int; d[1] = 1; return d }(),
			},
		},
		{
			name: "Full first chunk",
			setup: func() *BitSet {
				b := NewBitSet(128)
				for i := uint32(0); i < 32; i++ {
					b.Set(i)
				}
				return b
			},
			expected: ChunkEfficiencyStats{
				TotalChunks:       4,
				AccessedChunks:    1,
				AverageEfficiency: 1.0,
				Distribution:      func() [32]int { var d [32]int; d[31] = 1; return d }(),
			},
		},
		{
			name: "Mixed efficiency chunks",
			setup: func() *BitSet {
				b := NewBitSet(128)
				// First chunk: 8 bytes
				for i := uint32(0); i < 8; i++ {
					b.Set(i)
				}
				// Second chunk: 16 bytes
				for i := uint32(32); i < 48; i++ {
					b.Set(i)
				}
				// Third chunk: 32 bytes (full)
				for i := uint32(64); i < 96; i++ {
					b.Set(i)
				}
				return b
			},
			expected: ChunkEfficiencyStats{
				TotalChunks:       4,
				AccessedChunks:    3,
				AverageEfficiency: (8.0 + 16.0 + 32.0) / (3.0 * 32.0),
				Distribution:      func() [32]int { var d [32]int; d[8] = 1; d[16] = 1; d[31] = 1; return d }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.setup()
			stats := b.GetChunkEfficiencyStats()

			if stats.TotalChunks != tt.expected.TotalChunks {
				t.Errorf("TotalChunks: got %d, want %d", stats.TotalChunks, tt.expected.TotalChunks)
			}
			if stats.AccessedChunks != tt.expected.AccessedChunks {
				t.Errorf("AccessedChunks: got %d, want %d", stats.AccessedChunks, tt.expected.AccessedChunks)
			}
			if fmt.Sprintf("%.6f", stats.AverageEfficiency) != fmt.Sprintf("%.6f", tt.expected.AverageEfficiency) {
				t.Errorf("AverageEfficiency: got %.6f, want %.6f", stats.AverageEfficiency, tt.expected.AverageEfficiency)
			}

			// Check distribution
			if len(stats.Distribution) != len(tt.expected.Distribution) {
				t.Errorf("Distribution length: got %d, want %d", len(stats.Distribution), len(tt.expected.Distribution))
			}
			for k, v := range tt.expected.Distribution {
				if stats.Distribution[k] != v {
					t.Errorf("Distribution[%d]: got %d, want %d", k, stats.Distribution[k], v)
				}
			}
		})
	}
}

func TestGetChunkEfficiencies(t *testing.T) {
	b := NewBitSet(96) // 3 chunks

	// First chunk: 4 bytes (efficiency: 4/32 = 0.125)
	for i := uint32(0); i < 4; i++ {
		b.Set(i)
	}

	// Second chunk: 16 bytes (efficiency: 16/32 = 0.5)
	for i := uint32(32); i < 48; i++ {
		b.Set(i)
	}

	// Third chunk: not accessed

	efficiencies := b.GetChunkEfficiencies()

	if len(efficiencies) != 2 {
		t.Fatalf("Expected 2 efficiencies, got %d", len(efficiencies))
	}

	expected := []float64{0.125, 0.5}
	for i, eff := range efficiencies {
		if fmt.Sprintf("%.3f", eff) != fmt.Sprintf("%.3f", expected[i]) {
			t.Errorf("Efficiency[%d]: got %.3f, want %.3f", i, eff, expected[i])
		}
	}
}

func TestChunks(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *BitSet
		expected []byte
	}{
		{
			name: "Empty bitset",
			setup: func() *BitSet {
				return NewBitSet(128) // 4 chunks
			},
			expected: []byte{0, 0, 0, 0},
		},
		{
			name: "Single byte in first chunk",
			setup: func() *BitSet {
				b := NewBitSet(128)
				b.Set(0)
				return b
			},
			expected: []byte{1, 0, 0, 0},
		},
		{
			name: "Multiple bytes in different chunks",
			setup: func() *BitSet {
				b := NewBitSet(128)
				// First chunk: 3 bytes
				b.Set(0).Set(5).Set(10)
				// Second chunk: 2 bytes
				b.Set(35).Set(40)
				// Third chunk: 5 bytes
				b.Set(64).Set(65).Set(66).Set(67).Set(68)
				// Fourth chunk: 0 bytes (no access)
				return b
			},
			expected: []byte{3, 2, 5, 0},
		},
		{
			name: "Full first chunk",
			setup: func() *BitSet {
				b := NewBitSet(128)
				for i := uint32(0); i < 32; i++ {
					b.Set(i)
				}
				return b
			},
			expected: []byte{32, 0, 0, 0},
		},
		{
			name: "Mixed chunk usage",
			setup: func() *BitSet {
				b := NewBitSet(96) // 3 chunks
				// First chunk: 8 bytes
				for i := uint32(0); i < 8; i++ {
					b.Set(i)
				}
				// Second chunk: 16 bytes
				for i := uint32(32); i < 48; i++ {
					b.Set(i)
				}
				// Third chunk: 32 bytes (full)
				for i := uint32(64); i < 96; i++ {
					b.Set(i)
				}
				return b
			},
			expected: []byte{8, 16, 32},
		},
		{
			name: "Sparse access pattern",
			setup: func() *BitSet {
				b := NewBitSet(256) // 8 chunks
				// Access only specific bytes in different chunks
				b.Set(0)   // Chunk 0: 1 byte
				b.Set(63)  // Chunk 1: 1 byte
				b.Set(128) // Chunk 4: 1 byte
				b.Set(129) // Chunk 4: 2 bytes total
				b.Set(255) // Chunk 7: 1 byte
				return b
			},
			expected: []byte{1, 1, 0, 0, 2, 0, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.setup()
			chunks := b.Chunks()

			if len(chunks) != len(tt.expected) {
				t.Fatalf("Chunks length: got %d, want %d", len(chunks), len(tt.expected))
			}

			for i, expected := range tt.expected {
				if chunks[i] != expected {
					t.Errorf("Chunk[%d]: got %d, want %d", i, chunks[i], expected)
				}
			}
		})
	}
}

func TestEncodeChunks(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *BitSet
		expected string
	}{
		{
			name: "Empty bitset",
			setup: func() *BitSet {
				return NewBitSet(128) // 4 chunks
			},
			expected: base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 0}),
		},
		{
			name: "Single byte in first chunk",
			setup: func() *BitSet {
				b := NewBitSet(128)
				b.Set(0)
				return b
			},
			expected: base64.StdEncoding.EncodeToString([]byte{1, 0, 0, 0}),
		},
		{
			name: "Multiple bytes in different chunks",
			setup: func() *BitSet {
				b := NewBitSet(128)
				// First chunk: 3 bytes
				b.Set(0).Set(5).Set(10)
				// Second chunk: 2 bytes
				b.Set(35).Set(40)
				// Third chunk: 5 bytes
				b.Set(64).Set(65).Set(66).Set(67).Set(68)
				// Fourth chunk: 0 bytes (no access)
				return b
			},
			expected: base64.StdEncoding.EncodeToString([]byte{3, 2, 5, 0}),
		},
		{
			name: "Full first chunk",
			setup: func() *BitSet {
				b := NewBitSet(128)
				for i := uint32(0); i < 32; i++ {
					b.Set(i)
				}
				return b
			},
			expected: base64.StdEncoding.EncodeToString([]byte{32, 0, 0, 0}),
		},
		{
			name: "Mixed chunk usage",
			setup: func() *BitSet {
				b := NewBitSet(96) // 3 chunks
				// First chunk: 8 bytes
				for i := uint32(0); i < 8; i++ {
					b.Set(i)
				}
				// Second chunk: 16 bytes
				for i := uint32(32); i < 48; i++ {
					b.Set(i)
				}
				// Third chunk: 32 bytes (full)
				for i := uint32(64); i < 96; i++ {
					b.Set(i)
				}
				return b
			},
			expected: base64.StdEncoding.EncodeToString([]byte{8, 16, 32}),
		},
		{
			name: "Sparse access pattern",
			setup: func() *BitSet {
				b := NewBitSet(256) // 8 chunks
				// Access only specific bytes in different chunks
				b.Set(0)   // Chunk 0: 1 byte
				b.Set(63)  // Chunk 1: 1 byte
				b.Set(128) // Chunk 4: 1 byte
				b.Set(129) // Chunk 4: 2 bytes total
				b.Set(255) // Chunk 7: 1 byte
				return b
			},
			expected: base64.StdEncoding.EncodeToString([]byte{1, 1, 0, 0, 2, 0, 0, 1}),
		},
		{
			name: "Large contract with various patterns",
			setup: func() *BitSet {
				b := NewBitSet(512) // 16 chunks
				// Create a pattern: alternating high and low usage
				for i := 0; i < 16; i++ {
					if i%2 == 0 {
						// Even chunks: high usage (20 bytes)
						for j := 0; j < 20; j++ {
							b.Set(uint32(i*32 + j))
						}
					} else {
						// Odd chunks: low usage (3 bytes)
						for j := 0; j < 3; j++ {
							b.Set(uint32(i*32 + j))
						}
					}
				}
				return b
			},
			expected: base64.StdEncoding.EncodeToString([]byte{20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.setup()
			encoded := b.EncodeChunks()

			if encoded != tt.expected {
				t.Errorf("EncodeChunks: got %q, want %q", encoded, tt.expected)
			}

			// Verify that decoding gives us back the original chunks
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Fatalf("Failed to decode base64: %v", err)
			}

			chunks := b.Chunks()
			if len(decoded) != len(chunks) {
				t.Errorf("Decoded length: got %d, want %d", len(decoded), len(chunks))
			}

			for i, expected := range chunks {
				if i < len(decoded) && decoded[i] != expected {
					t.Errorf("Decoded chunk[%d]: got %d, want %d", i, decoded[i], expected)
				}
			}
		})
	}
}

func TestGetChunkDetails(t *testing.T) {
	b := NewBitSet(96) // 3 chunks

	// First chunk: 8 bytes
	for i := uint32(0); i < 8; i++ {
		b.Set(i)
	}

	// Skip second chunk

	// Third chunk: 24 bytes
	for i := uint32(64); i < 88; i++ {
		b.Set(i)
	}

	details := b.GetChunkDetails()

	if len(details) != 2 {
		t.Fatalf("Expected 2 chunk details, got %d", len(details))
	}

	// Check first chunk
	if details[0].Index != 0 {
		t.Errorf("First chunk index: got %d, want 0", details[0].Index)
	}
	if details[0].BytesAccessed != 8 {
		t.Errorf("First chunk bytes accessed: got %d, want 8", details[0].BytesAccessed)
	}
	if fmt.Sprintf("%.3f", details[0].Efficiency) != "0.250" {
		t.Errorf("First chunk efficiency: got %.3f, want 0.250", details[0].Efficiency)
	}

	// Check third chunk
	if details[1].Index != 2 {
		t.Errorf("Second chunk index: got %d, want 2", details[1].Index)
	}
	if details[1].BytesAccessed != 24 {
		t.Errorf("Second chunk bytes accessed: got %d, want 24", details[1].BytesAccessed)
	}
	if fmt.Sprintf("%.3f", details[1].Efficiency) != "0.750" {
		t.Errorf("Second chunk efficiency: got %.3f, want 0.750", details[1].Efficiency)
	}
}
