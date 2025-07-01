# Ethereum Code Chunking Analysis

A comprehensive analysis of code chunking patterns on Ethereum mainnet to understand bytecode utilization efficiency and the impact of code operations on access patterns.

## Overview

This project consists of two main components:
1. **Go-based data collection tool** - Extracts bytecode access patterns from Ethereum blocks
2. **Python-based analysis notebook** - Analyzes the collected data and generates insights

The analysis examines 2.46 million contract interactions across 9,993 blocks (22000000-22010000) to answer key research questions about bytecode chunking efficiency.

## Prerequisites

### For Data Collection (Go)
- Go 1.19 or higher
- Access to Ethereum RPC endpoint
- 4GB+ RAM for processing large blocks

### For Analysis (Python)
- Python 3.8 or higher
- 8GB+ RAM (recommended for handling large datasets)
- Jupyter Notebook environment

## Installation & Setup

### Option 1: Full Setup (Data Collection + Analysis)

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd chunk-analysis
   ```

2. **Setup Go environment**:
   ```bash
   # Install Go dependencies
   go mod download
   
   # Build the data collection tool
   make build
   # or manually: go build -o bin/chunk-analyzer main.go
   ```

3. **Setup Python environment**:
   ```bash
   # Create virtual environment
   python -m venv venv
   source venv/bin/activate  # On macOS/Linux
   # venv\Scripts\activate   # On Windows
   
   # Install Python dependencies
   pip install -r requirements.txt
   ```

### Option 2: Analysis Only (Using Existing Data)

If you only want to run the analysis with existing CSV files:

1. **Setup Python environment only**:
   ```bash
   python -m venv venv
   source venv/bin/activate
   pip install pandas numpy matplotlib seaborn jupyter
   ```

2. **Ensure data files are present**:
   ```bash
   ls results/
   # Should show: analysis-0.csv analysis-1.csv
   ```

## Configuration

### Environment Setup for Data Collection

1. **Create configuration file**:
   ```bash
   cp configs/config.env.example configs/config.env
   ```

2. **Edit configuration** (`configs/config.env`):
   ```env
    RPC_URLS=RPC_URL1,[...optional]
    START_BLOCKS=22000000,[...optional]
    END_BLOCKS=22005000,[...optional]
    RESULT_DIR=results
   ```

## Usage

### Step 1: Data Collection (Optional)

If you need to collect new data:

1. **Configure RPC endpoint**:
   ```bash
   # Edit configs/config.env with your Ethereum RPC URL
   export RPC_URL="https://your-rpc-endpoint.com"
   ```

2. **Run data collection**:
   ```bash
   make build
   ./bin/chunk-analyzer run
   ```

### Step 2: Data Analysis

1. **Start Jupyter Notebook**:
   ```bash
   # Activate Python environment
   source venv/bin/activate
   
   # Start Jupyter
   jupyter notebook
   # or: jupyter lab
   ```

2. **Open the analysis notebook**:
   Navigate to `analysis/ethereum_code_chunking_analysis.ipynb`

3. **Run the analysis**:
   - Execute all cells: `Kernel` � `Restart & Run All`
   - Or run cells individually: `Shift + Enter`

### Expected Runtime

**Data Collection** (if running):
- ~10-20 seconds per block
- Total: 24-48 hours for 10,000 blocks
- Memory: 2-4 GB peak usage

## Data Schema

### Input Data (CSV Format)

Each row represents a contract interaction with the following columns:

| Column | Type | Description |
|--------|------|-------------|
| `block_number` | int64 | Block number on Ethereum mainnet |
| `address` | string | Contract address (hex string) |
| `bytecode_size` | int64 | Total size of contract bytecode in bytes |
| `bytes_count` | int64 | Number of bytes accessed during execution |
| `chunks_count` | int64 | Number of 32-byte chunks accessed |
| `code_ops_count` | int64 | Number of code operations (EXTCODESIZE, EXTCODECOPY, EXTCODEHASH, CODECOPY, CODESIZE) |

### Sample Data

```csv
block_number,address,bytecode_size,bytes_count,chunks_count,code_ops_count
22000000,0x8ad599c3A0ff1De082011EFDDc58f1908eb6e6D8,22142,5136,215,1
22000000,0x85a471E728F8F0932694d349993DC9A599a5978c,19080,2394,93,0
22000000,0x5Ebac8dbfbBA22168471b0f914131d1976536A25,11329,2762,114,0
```