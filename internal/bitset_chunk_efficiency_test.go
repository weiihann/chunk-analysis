package internal

import (
	"encoding/base64"
	"testing"
)

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
