package internal

import (
	"testing"
)

func TestNewBitSet(t *testing.T) {
	tests := []struct {
		name        string
		size        uint32
		expected    uint32
		shouldPanic bool
	}{
		{
			name:     "valid size one",
			size:     1,
			expected: 1,
		},
		{
			name:     "valid size 64",
			size:     64,
			expected: 64,
		},
		{
			name:     "valid size 65",
			size:     65,
			expected: 65,
		},
		{
			name:     "valid size max contract bytes",
			size:     maxContractBytes,
			expected: maxContractBytes,
		},
		{
			name:        "invalid size exceeds max",
			size:        maxContractBytes + 1,
			shouldPanic: true,
		},
		{
			name:        "invalid size way too large",
			size:        100000,
			shouldPanic: true,
		},
		{
			name:        "invalid size zero",
			size:        0,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("NewBitSet() should have panicked for size %d", tt.size)
					}
				}()
			}

			bs := NewBitSet(tt.size)

			if !tt.shouldPanic {
				if bs == nil {
					t.Fatal("NewBitSet() returned nil")
				}
				if bs.size != tt.expected {
					t.Errorf("NewBitSet() size = %d, expected %d", bs.size, tt.expected)
				}
				if bs.Count() != 0 {
					t.Errorf("NewBitSet() should start with count 0, got %d", bs.Count())
				}
				if bs.Proportion() != 0.0 {
					t.Errorf("NewBitSet() should start with proportion 0.0, got %f", bs.Proportion())
				}
			}
		})
	}
}

func TestBitSet_Set(t *testing.T) {
	tests := []struct {
		name          string
		size          uint32
		setIndexes    []uint32
		expectedCount int
		shouldPanic   []bool // corresponds to setIndexes
	}{
		{
			name:          "set single bit in small bitset",
			size:          10,
			setIndexes:    []uint32{0},
			expectedCount: 1,
			shouldPanic:   []bool{false},
		},
		{
			name:          "set multiple bits in small bitset",
			size:          10,
			setIndexes:    []uint32{0, 5, 9},
			expectedCount: 3,
			shouldPanic:   []bool{false, false, false},
		},
		{
			name:          "set same bit multiple times",
			size:          10,
			setIndexes:    []uint32{5, 5, 5},
			expectedCount: 1,
			shouldPanic:   []bool{false, false, false},
		},
		{
			name:          "set bits across word boundaries",
			size:          100,
			setIndexes:    []uint32{0, 31, 32, 99},
			expectedCount: 4,
			shouldPanic:   []bool{false, false, false, false},
		},
		{
			name:          "set all bits in single word",
			size:          32,
			setIndexes:    make([]uint32, 32),
			expectedCount: 32,
		},
		{
			name:          "set out of bounds",
			size:          10,
			setIndexes:    []uint32{10},
			expectedCount: 0,
			shouldPanic:   []bool{true},
		},
		{
			name:          "set way out of bounds",
			size:          10,
			setIndexes:    []uint32{100},
			expectedCount: 0,
			shouldPanic:   []bool{true},
		},
	}

	// Initialize the "set all bits in single word" test case
	for i := range tests[4].setIndexes {
		tests[4].setIndexes[i] = uint32(i)
	}
	tests[4].shouldPanic = make([]bool, 32)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := NewBitSet(tt.size)

			for i, index := range tt.setIndexes {
				shouldPanic := i < len(tt.shouldPanic) && tt.shouldPanic[i]

				if shouldPanic {
					func() {
						defer func() {
							if r := recover(); r == nil {
								t.Errorf("Set(%d) should have panicked", index)
							}
						}()
						bs.Set(index)
					}()
				} else {
					result := bs.Set(index)
					if result != bs {
						t.Error("Set() should return the same BitSet instance for method chaining")
					}
				}
			}

			if tt.expectedCount > 0 {
				if bs.Count() != tt.expectedCount {
					t.Errorf("Count() = %d, expected %d", bs.Count(), tt.expectedCount)
				}
			}
		})
	}
}

func TestBitSet_Count(t *testing.T) {
	tests := []struct {
		name          string
		size          uint32
		setIndexes    []uint32
		expectedCount int
	}{
		{
			name:          "empty bitset",
			size:          10,
			setIndexes:    []uint32{},
			expectedCount: 0,
		},
		{
			name:          "single bit set",
			size:          32,
			setIndexes:    []uint32{0},
			expectedCount: 1,
		},
		{
			name:          "multiple bits in same word",
			size:          32,
			setIndexes:    []uint32{0, 1, 2, 10, 31},
			expectedCount: 5,
		},
		{
			name:          "bits across multiple words",
			size:          200,
			setIndexes:    []uint32{0, 31, 32, 63, 64, 199},
			expectedCount: 6,
		},
		{
			name:          "all bits in single word",
			size:          32,
			setIndexes:    make([]uint32, 32),
			expectedCount: 32,
		},
		{
			name:          "sparse bits across large bitset",
			size:          1000,
			setIndexes:    []uint32{0, 100, 200, 300, 400, 500, 600, 700, 800, 999},
			expectedCount: 10,
		},
	}

	// Initialize the "all bits in single word" test case
	for i := range tests[4].setIndexes {
		tests[4].setIndexes[i] = uint32(i)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := NewBitSet(tt.size)

			for _, index := range tt.setIndexes {
				bs.Set(index)
			}

			count := bs.Count()
			if count != tt.expectedCount {
				t.Errorf("Count() = %d, expected %d", count, tt.expectedCount)
			}
		})
	}
}

func TestBitSet_Proportion(t *testing.T) {
	tests := []struct {
		name               string
		size               uint32
		setIndexes         []uint32
		expectedProportion float64
		tolerance          float64
	}{
		{
			name:               "empty bitset",
			size:               10,
			setIndexes:         []uint32{},
			expectedProportion: 0.0,
			tolerance:          0.0,
		},
		{
			name:               "single bit in small bitset",
			size:               10,
			setIndexes:         []uint32{0},
			expectedProportion: 0.1,
			tolerance:          0.001,
		},
		{
			name:               "half bits set",
			size:               10,
			setIndexes:         []uint32{0, 1, 2, 3, 4},
			expectedProportion: 0.5,
			tolerance:          0.001,
		},
		{
			name:               "all bits set",
			size:               10,
			setIndexes:         []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			expectedProportion: 1.0,
			tolerance:          0.001,
		},
		{
			name:               "one bit in large bitset",
			size:               1000,
			setIndexes:         []uint32{500},
			expectedProportion: 0.001,
			tolerance:          0.0001,
		},
		{
			name:               "size of 1 with bit set",
			size:               1,
			setIndexes:         []uint32{0},
			expectedProportion: 1.0,
			tolerance:          0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := NewBitSet(tt.size)

			for _, index := range tt.setIndexes {
				bs.Set(index)
			}

			proportion := bs.Proportion()
			if abs(proportion-tt.expectedProportion) > tt.tolerance {
				t.Errorf("Proportion() = %f, expected %f (±%f)", proportion, tt.expectedProportion, tt.tolerance)
			}
		})
	}
}

func TestBitSet_ChunkCount(t *testing.T) {
	tests := []struct {
		name               string
		size               uint32
		setIndexes         []uint32
		expectedChunkCount int
	}{
		{
			name:               "empty bitset",
			size:               64,
			setIndexes:         []uint32{},
			expectedChunkCount: 0,
		},
		{
			name:               "single bit in first chunk",
			size:               64,
			setIndexes:         []uint32{0},
			expectedChunkCount: 1,
		},
		{
			name:               "multiple bits in same chunk",
			size:               64,
			setIndexes:         []uint32{0, 1, 15, 31},
			expectedChunkCount: 1,
		},
		{
			name:               "bits in first two chunks",
			size:               64,
			setIndexes:         []uint32{0, 32},
			expectedChunkCount: 2,
		},
		{
			name:               "bits across multiple chunks",
			size:               128,
			setIndexes:         []uint32{0, 32, 64, 96},
			expectedChunkCount: 4,
		},
		{
			name:               "sparse bits across many chunks",
			size:               200,
			setIndexes:         []uint32{10, 50, 90, 130, 170},
			expectedChunkCount: 5,
		},
		{
			name:               "all chunks with at least one bit",
			size:               96,
			setIndexes:         []uint32{0, 32, 64},
			expectedChunkCount: 3,
		},
		{
			name:               "single bit in last chunk",
			size:               100,
			setIndexes:         []uint32{99},
			expectedChunkCount: 1,
		},
		{
			name:               "bits in non-consecutive chunks",
			size:               160,
			setIndexes:         []uint32{5, 100, 155},
			expectedChunkCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := NewBitSet(tt.size)

			for _, index := range tt.setIndexes {
				bs.Set(index)
			}

			chunkCount := bs.ChunkCount()
			if chunkCount != tt.expectedChunkCount {
				t.Errorf("ChunkCount() = %d, expected %d", chunkCount, tt.expectedChunkCount)
			}
		})
	}
}

func TestBitSet_ChunkProportion(t *testing.T) {
	tests := []struct {
		name                    string
		size                    uint32
		setIndexes              []uint32
		expectedChunkProportion float64
		tolerance               float64
	}{
		{
			name:                    "empty bitset",
			size:                    64,
			setIndexes:              []uint32{},
			expectedChunkProportion: 0.0,
			tolerance:               0.0,
		},
		{
			name:                    "single chunk out of two",
			size:                    64,
			setIndexes:              []uint32{0},
			expectedChunkProportion: 0.5,
			tolerance:               0.001,
		},
		{
			name:                    "both chunks out of two",
			size:                    64,
			setIndexes:              []uint32{0, 32},
			expectedChunkProportion: 1.0,
			tolerance:               0.001,
		},
		{
			name:                    "half chunks in large bitset",
			size:                    128,
			setIndexes:              []uint32{0, 64},
			expectedChunkProportion: 0.5,
			tolerance:               0.001,
		},
		{
			name:                    "all chunks in large bitset",
			size:                    128,
			setIndexes:              []uint32{0, 32, 64, 96},
			expectedChunkProportion: 1.0,
			tolerance:               0.001,
		},
		{
			name:                    "single chunk in large bitset",
			size:                    320,
			setIndexes:              []uint32{100},
			expectedChunkProportion: 0.1,
			tolerance:               0.001,
		},
		{
			name:                    "sparse chunks",
			size:                    320,
			setIndexes:              []uint32{10, 100, 200},
			expectedChunkProportion: 0.3,
			tolerance:               0.001,
		},
		{
			name:                    "single bit in single chunk bitset",
			size:                    10,
			setIndexes:              []uint32{5},
			expectedChunkProportion: 1.0,
			tolerance:               0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := NewBitSet(tt.size)

			for _, index := range tt.setIndexes {
				bs.Set(index)
			}

			chunkProportion := bs.ChunkProportion()
			if abs(chunkProportion-tt.expectedChunkProportion) > tt.tolerance {
				t.Errorf("ChunkProportion() = %f, expected %f (±%f)", chunkProportion, tt.expectedChunkProportion, tt.tolerance)
			}
		})
	}
}

func TestBitSet_ChunkMethods_EdgeCases(t *testing.T) {
	t.Run("single chunk bitset", func(t *testing.T) {
		bs := NewBitSet(20)

		// Initially empty
		if bs.ChunkCount() != 0 {
			t.Errorf("Empty single chunk bitset should have ChunkCount 0, got %d", bs.ChunkCount())
		}
		if bs.ChunkProportion() != 0.0 {
			t.Errorf("Empty single chunk bitset should have ChunkProportion 0.0, got %f", bs.ChunkProportion())
		}

		// Set a bit
		bs.Set(10)
		if bs.ChunkCount() != 1 {
			t.Errorf("Single chunk bitset with bit set should have ChunkCount 1, got %d", bs.ChunkCount())
		}
		if bs.ChunkProportion() != 1.0 {
			t.Errorf("Single chunk bitset with bit set should have ChunkProportion 1.0, got %f", bs.ChunkProportion())
		}
	})

	t.Run("chunk boundaries", func(t *testing.T) {
		bs := NewBitSet(100)

		// Set bits at chunk boundaries
		bs.Set(31) // Last bit of first chunk
		bs.Set(32) // First bit of second chunk
		bs.Set(63) // Last bit of second chunk
		bs.Set(64) // First bit of third chunk

		expectedChunks := 3
		if bs.ChunkCount() != expectedChunks {
			t.Errorf("ChunkCount() = %d, expected %d", bs.ChunkCount(), expectedChunks)
		}

		// BitSet size 100 means (100+31)/32 = 4 chunks total
		expectedProportion := 3.0 / 4.0
		if abs(bs.ChunkProportion()-expectedProportion) > 0.001 {
			t.Errorf("ChunkProportion() = %f, expected %f", bs.ChunkProportion(), expectedProportion)
		}
	})

	t.Run("maximum size bitset chunks", func(t *testing.T) {
		bs := NewBitSet(maxContractBytes)

		// Set first and last bits
		bs.Set(0)
		bs.Set(maxContractBytes - 1)

		expectedChunks := 2
		if bs.ChunkCount() != expectedChunks {
			t.Errorf("Should have ChunkCount %d, got %d", expectedChunks, bs.ChunkCount())
		}

		totalChunks := (maxContractBytes + 31) / 32
		expectedProportion := 2.0 / float64(totalChunks)
		if abs(bs.ChunkProportion()-expectedProportion) > 0.000001 {
			t.Errorf("ChunkProportion should be %f, got %f", expectedProportion, bs.ChunkProportion())
		}
	})
}

func TestBitSet_EdgeCases(t *testing.T) {
	t.Run("maximum size bitset", func(t *testing.T) {
		bs := NewBitSet(maxContractBytes)
		if bs.size != maxContractBytes {
			t.Errorf("Max size bitset should have size %d, got %d", maxContractBytes, bs.size)
		}

		// Set first and last bits
		bs.Set(0)
		bs.Set(maxContractBytes - 1)

		if bs.Count() != 2 {
			t.Errorf("Should have count 2, got %d", bs.Count())
		}

		expectedProportion := 2.0 / float64(maxContractBytes)
		if abs(bs.Proportion()-expectedProportion) > 0.000001 {
			t.Errorf("Proportion should be %f, got %f", expectedProportion, bs.Proportion())
		}
	})

	t.Run("method chaining", func(t *testing.T) {
		bs := NewBitSet(10)
		result := bs.Set(0).Set(1).Set(2)

		if result != bs {
			t.Error("Set() method chaining should return the same instance")
		}
		if bs.Count() != 3 {
			t.Errorf("After chaining Set(0).Set(1).Set(2), count should be 3, got %d", bs.Count())
		}
	})
}

func TestBitSet_WordBoundaries(t *testing.T) {
	t.Run("bits around 32-bit word boundaries", func(t *testing.T) {
		bs := NewBitSet(200)

		// Set bits around word boundaries
		testIndexes := []uint32{
			0, 1, 30, 31, // First word
			32, 33, 62, 63, // Second word
			64, 65, 94, 95, // Third word
		}

		for _, index := range testIndexes {
			bs.Set(index)
		}

		if bs.Count() != len(testIndexes) {
			t.Errorf("Count should be %d, got %d", len(testIndexes), bs.Count())
		}

		expectedProportion := float64(len(testIndexes)) / 200.0
		if abs(bs.Proportion()-expectedProportion) > 0.001 {
			t.Errorf("Proportion should be %f, got %f", expectedProportion, bs.Proportion())
		}
	})
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
