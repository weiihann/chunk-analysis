package internal

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
)

type ResultWriter struct {
	file     *os.File
	writer   *csv.Writer
	filePath string
}

func NewResultWriter(dir string, id int) *ResultWriter {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(fmt.Errorf("failed to create directory: %w", err))
	}
	return &ResultWriter{
		filePath: filepath.Join(dir, fmt.Sprintf("analysis-%d.csv", id)),
	}
}

func (w *ResultWriter) Write(blockNum uint64, results map[common.Address]*MergedTraceResult) error {
	// Initialize the file and CSV writer if not already done
	if w.file == nil {
		if err := w.initializeFile(); err != nil {
			return fmt.Errorf("failed to initialize file: %w", err)
		}
	}

	// Write each address result to the CSV
	for address, result := range results {
		record := []string{
			strconv.FormatUint(blockNum, 10),                   // block number
			address.Hex(),                                      // address
			strconv.FormatUint(uint64(result.Bits.Size()), 10), // bytecode size
			strconv.Itoa(result.Bits.Count()),                  // bytes count
			strconv.Itoa(result.Bits.ChunkCount()),             // chunks count
			strconv.Itoa(result.CodeSizeHashCount),             // code size hash count
			strconv.Itoa(result.CodeCopyCount),                 // code copy count
		}

		if err := w.writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	// Flush the writer to ensure data is written to disk
	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV writer: %w", err)
	}

	return nil
}

func (w *ResultWriter) initializeFile() error {
	// Check if file already exists
	fileExists := false
	if _, err := os.Stat(w.filePath); err == nil {
		fileExists = true
	}

	var file *os.File
	var err error

	if fileExists {
		// Open existing file in append mode
		file, err = os.OpenFile(w.filePath, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open existing file: %w", err)
		}

		// Ensure the file ends with a newline before appending
		// Check if file is empty or doesn't end with newline
		stat, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file stats: %w", err)
		}

		if stat.Size() > 0 {
			// Write a newline to ensure proper separation
			if _, err := file.Write([]byte("\n")); err != nil {
				return fmt.Errorf("failed to write newline separator: %w", err)
			}
		}
	} else {
		// Create new file
		file, err = os.Create(w.filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}

	w.file = file
	w.writer = csv.NewWriter(file)
	w.writer.UseCRLF = false

	// Write header row only for new files
	if !fileExists {
		header := []string{"block_number", "address", "bytecode_size", "bytes_count", "chunks_count", "code_size_hash_count", "code_copy_count"}
		if err := w.writer.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
		w.writer.Flush()
		if err := w.writer.Error(); err != nil {
			return fmt.Errorf("failed to flush header: %w", err)
		}
	}

	return nil
}

// Close closes the CSV file and writer safely
func (w *ResultWriter) Close() error {
	if w.writer != nil {
		w.writer.Flush()
		if err := w.writer.Error(); err != nil {
			return fmt.Errorf("failed to flush writer on close: %w", err)
		}
	}

	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
		w.file = nil
		w.writer = nil
	}

	return nil
}
