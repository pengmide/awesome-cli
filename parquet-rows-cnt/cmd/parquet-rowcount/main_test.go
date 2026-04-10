package main

import (
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"
)

type testRow struct {
	ID   int64  `parquet:"id"`
	Name string `parquet:"name"`
}

func TestCountRows(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.parquet")
	rows := []testRow{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
		{ID: 3, Name: "carol"},
	}

	if err := parquet.WriteFile(path, rows); err != nil {
		t.Fatalf("write parquet file: %v", err)
	}

	got, err := countRows(path)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}

	if got != int64(len(rows)) {
		t.Fatalf("row count mismatch: got %d want %d", got, len(rows))
	}
}
