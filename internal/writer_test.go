package internal

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestNewResultWriter(t *testing.T) {
	dir := "test_output"
	writer := NewResultWriter(dir, 0)

	if writer == nil {
		t.Fatal("NewResultWriter() returned nil")
	}

	if writer.file != nil {
		t.Error("Expected file to be nil initially")
	}

	if writer.writer != nil {
		t.Error("Expected writer to be nil initially")
	}
}

func TestResultWriter_Write_SingleResult(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewResultWriter(tempDir, 0)
	defer writer.Close()

	// Create test data
	blockNum := uint64(12345)
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	bitSet := NewBitSet(100)
	bitSet.Set(10).Set(20).Set(30)

	results := map[common.Address]*MergedTraceResult{
		addr: {
			Bits:          bitSet,
			CodeSizeCount: 5,
			CodeCopyCount: 1,
		},
	}

	// Write the data
	err := writer.Write(blockNum, results)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify file was created and contains correct data
	expectedPath := filepath.Join(tempDir, "analysis-0.csv")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatal("CSV file was not created")
	}

	// Read and verify the CSV content
	file, err := os.Open(expectedPath)
	if err != nil {
		t.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	// Should have header + 1 data row
	if len(records) != 2 {
		t.Fatalf("Expected 2 rows (header + data), got %d", len(records))
	}

	// Verify header
	expectedHeader := []string{"block_number", "address", "bytecode_size", "chunks_data", "code_size_count", "code_copy_count"}
	if !equalSlices(records[0], expectedHeader) {
		t.Errorf("Header mismatch. Expected %v, got %v", expectedHeader, records[0])
	}

	// Verify data row
	expectedData := []string{"12345", strings.ToLower(addr.Hex()), strconv.Itoa(int(bitSet.Size())), bitSet.EncodeChunks(), "5", "1"}
	if !equalSlices(records[1], expectedData) {
		t.Errorf("Data row mismatch. Expected %v, got %v", expectedData, records[1])
	}
}

func TestResultWriter_Write_MultipleResults(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewResultWriter(tempDir, 0)
	defer writer.Close()

	blockNum := uint64(67890)

	// Create multiple test addresses with different data
	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	bitSet1 := NewBitSet(50)
	bitSet1.Set(0).Set(1).Set(2)

	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	bitSet2 := NewBitSet(200)
	bitSet2.Set(10).Set(50).Set(100).Set(150)

	results := map[common.Address]*MergedTraceResult{
		addr1: {
			Bits:          bitSet1,
			CodeSizeCount: 3,
			CodeCopyCount: 0,
		},
		addr2: {
			Bits:          bitSet2,
			CodeSizeCount: 7,
			CodeCopyCount: 0,
		},
	}

	err := writer.Write(blockNum, results)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Read and verify the CSV content
	expectedPath := filepath.Join(tempDir, "analysis-0.csv")
	file, err := os.Open(expectedPath)
	if err != nil {
		t.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	// Should have header + 2 data rows
	if len(records) != 3 {
		t.Fatalf("Expected 3 rows (header + 2 data), got %d", len(records))
	}

	// Verify both addresses are present (order may vary due to map iteration)
	foundAddrs := make(map[string]bool)
	for i := 1; i < len(records); i++ {
		foundAddrs[records[i][1]] = true
		// Verify block number is correct for all rows
		if records[i][0] != "67890" {
			t.Errorf("Expected block number 67890, got %s", records[i][0])
		}
	}

	if !foundAddrs[strings.ToLower(addr1.Hex())] {
		t.Error("Address 1 not found in results")
	}
	if !foundAddrs[strings.ToLower(addr2.Hex())] {
		t.Error("Address 2 not found in results")
	}
}

func TestResultWriter_Write_MultipleCalls(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewResultWriter(tempDir, 0)
	defer writer.Close()

	addr := common.HexToAddress("0x3333333333333333333333333333333333333333")
	bitSet := NewBitSet(10)
	bitSet.Set(5)

	results := map[common.Address]*MergedTraceResult{
		addr: {
			Bits:          bitSet,
			CodeSizeCount: 1,
			CodeCopyCount: 0,
		},
	}

	// Write multiple blocks
	for blockNum := uint64(1); blockNum <= 3; blockNum++ {
		err := writer.Write(blockNum, results)
		if err != nil {
			t.Fatalf("Write() failed for block %d: %v", blockNum, err)
		}
	}

	// Read and verify the CSV content
	expectedPath := filepath.Join(tempDir, "analysis-0.csv")
	file, err := os.Open(expectedPath)
	if err != nil {
		t.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	// Should have header + 3 data rows
	if len(records) != 4 {
		t.Fatalf("Expected 4 rows (header + 3 data), got %d", len(records))
	}

	// Verify block numbers are in sequence
	expectedBlockNums := []string{"1", "2", "3"}
	for i, expectedBlockNum := range expectedBlockNums {
		if records[i+1][0] != expectedBlockNum {
			t.Errorf("Expected block number %s, got %s", expectedBlockNum, records[i+1][0])
		}
	}
}

func TestResultWriter_Write_EmptyResults(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewResultWriter(tempDir, 0)
	defer writer.Close()

	blockNum := uint64(999)
	results := map[common.Address]*MergedTraceResult{}

	err := writer.Write(blockNum, results)
	if err != nil {
		t.Fatalf("Write() failed for empty results: %v", err)
	}

	// File should still be created with header
	expectedPath := filepath.Join(tempDir, "analysis-0.csv")
	file, err := os.Open(expectedPath)
	if err != nil {
		t.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	// Should have only header
	if len(records) != 1 {
		t.Fatalf("Expected 1 row (header only), got %d", len(records))
	}
}

func TestResultWriter_Close(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewResultWriter(tempDir, 0)

	// Write some data first
	blockNum := uint64(123)
	addr := common.HexToAddress("0x4444444444444444444444444444444444444444")
	bitSet := NewBitSet(10)
	bitSet.Set(1)

	results := map[common.Address]*MergedTraceResult{
		addr: {
			Bits:          bitSet,
			CodeSizeCount: 1,
			CodeCopyCount: 0,
		},
	}

	err := writer.Write(blockNum, results)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Close the writer
	err = writer.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Verify internal state is reset
	if writer.file != nil {
		t.Error("Expected file to be nil after Close()")
	}
	if writer.writer != nil {
		t.Error("Expected writer to be nil after Close()")
	}

	// Verify file still exists and is readable
	expectedPath := filepath.Join(tempDir, "analysis-0.csv")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatal("CSV file was removed after Close()")
	}
}

func TestResultWriter_Close_WithoutWrite(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewResultWriter(tempDir, 0)

	// Close without writing anything
	err := writer.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Should not create any files
	expectedPath := filepath.Join(tempDir, "analysis-0.csv")
	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Error("CSV file should not exist when closing without writing")
	}
}

func TestResultWriter_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "nested", "deep", "directory")
	writer := NewResultWriter(nestedDir, 0)
	defer writer.Close()

	blockNum := uint64(1)
	addr := common.HexToAddress("0x6666666666666666666666666666666666666666")
	bitSet := NewBitSet(5)
	bitSet.Set(0)

	results := map[common.Address]*MergedTraceResult{
		addr: {
			Bits:          bitSet,
			CodeSizeCount: 1,
			CodeCopyCount: 0,
		},
	}

	err := writer.Write(blockNum, results)
	if err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Fatal("Nested directory was not created")
	}

	// Verify file was created in the correct location
	expectedPath := filepath.Join(nestedDir, "analysis-0.csv")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatal("CSV file was not created in nested directory")
	}
}

func TestResultWriter_LargeData(t *testing.T) {
	tempDir := t.TempDir()
	writer := NewResultWriter(tempDir, 0)
	defer writer.Close()

	blockNum := uint64(1)

	// Create a large bitset
	bitSet := NewBitSet(1000)
	for i := uint64(0); i < 500; i++ {
		bitSet.Set(uint32(i))
	}

	addr := common.HexToAddress("0x7777777777777777777777777777777777777777")
	results := map[common.Address]*MergedTraceResult{
		addr: {
			Bits:          bitSet,
			CodeSizeCount: 999,
			CodeCopyCount: 0,
		},
	}

	err := writer.Write(blockNum, results)
	if err != nil {
		t.Fatalf("Write() failed for large data: %v", err)
	}

	// Verify the data was written correctly
	expectedPath := filepath.Join(tempDir, "analysis-0.csv")
	file, err := os.Open(expectedPath)
	if err != nil {
		t.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(records))
	}

	// Verify the large numbers were written correctly
	expectedData := []string{"1", strings.ToLower(addr.Hex()), strconv.Itoa(int(bitSet.Size())), bitSet.EncodeChunks(), "999", "0", "0"}
	if !equalSlices(records[1], expectedData) {
		t.Errorf("Large data row mismatch. Expected %v, got %v", expectedData, records[1])
	}
}

// Helper function to compare string slices
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
