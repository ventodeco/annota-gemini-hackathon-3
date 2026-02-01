# Knowledge Service

CSV-based vocabulary lookup service that enriches AI annotations with domain-specific terminology.

## Overview

The Knowledge Service loads Japanese business/work vocabulary from a CSV file at startup and provides fast lookups to enhance Gemini prompts with contextual information.

## Architecture

```
User selects text → POST /v1/ai/analyze
                           ↓
                  ┌─────────────────┐
                  │ Knowledge Lookup │
                  │  1. Exact match  │
                  │  2. Substring    │
                  └─────────────────┘
                           ↓
                  Found? → Inject into Gemini prompt
                           ↓
                     Gemini API
```

## CSV Format

| Column | Example | Description |
|--------|---------|-------------|
| Kosakata | 請求 | Japanese term (kanji) - **primary lookup key** |
| Kana | せいきゅう | Hiragana reading |
| Arti (EN / ID) | claim, meminta | Meaning in English/Indonesian |
| Cara Baca | seikyuu | Romaji pronunciation |
| Deskripsi | permintaan... | Detailed description |
| Bidang Pekerjaan | Perbankan, Bisnis | Work fields |
| Industri | Financial | Industry categories |
| Konteks | (optional) | Additional context |

**Note:** Notion links in Bidang Pekerjaan and Industri columns are automatically stripped.

## Lookup Algorithm

Two-phase search strategy:

### Phase 1: Exact Match (O(1))
Uses hash map index by `Kosakata` field.

### Phase 2: Substring Match (O(n))
For each entry, checks:
1. **Text contains term** - `"請求書を送る"` contains `"請求"` ✓
2. **Term contains text** - `"請求書"` contains `"請求"` ✓

### Example

```
Input: "請求書を送る"

Phase 1: index["請求書を送る"] → miss

Phase 2: Scan all entries
  • "請求" in "請求書を送る"? ✓ (match)
  • "請求書" in "請求書を送る"? ✓ (match)
  • "見積もり" in text? ✗

Result: [請求, 請求書]
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `KNOWLEDGE_CSV_PATH` | `data/knowledge.csv` | Path to CSV file |

## Usage

Place CSV file at configured path. Server logs on startup:
- Success: `Loaded knowledge CSV from data/knowledge.csv`
- Missing file: `Warning: Failed to load knowledge CSV... Continuing without knowledge context.`

## Code Location

- `internal/knowledge/knowledge.go` - Types and interface
- `internal/knowledge/csv_loader.go` - CSV parsing and lookup
- `internal/knowledge/knowledge_test.go` - Unit tests
