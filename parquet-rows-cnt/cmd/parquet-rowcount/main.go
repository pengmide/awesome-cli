package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/parquet-go/parquet-go"
)

func main() {
	var (
		showTotalOnly bool
		quiet         bool
	)

	flag.BoolVar(&showTotalOnly, "total-only", false, "only print the total row count")
	flag.BoolVar(&quiet, "q", false, "suppress per-file errors and continue")
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "Usage: %s [flags] <file.parquet> [more files...]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintln(w, "Count rows from Parquet file metadata without scanning the full file.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Flags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var total int64
	hadError := false

	for _, path := range paths {
		rows, err := countRows(path)
		if err != nil {
			hadError = true
			if !quiet {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			}
			continue
		}

		total += rows
		if !showTotalOnly {
			fmt.Printf("%d\t%s\n", rows, path)
		}
	}

	if showTotalOnly || len(paths) > 1 {
		fmt.Printf("%d\ttotal\n", total)
	}

	if hadError {
		os.Exit(1)
	}
}

func countRows(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if stat.IsDir() {
		return 0, errors.New("path is a directory")
	}

	readerAt := io.NewSectionReader(file, 0, stat.Size())
	pf, err := parquet.OpenFile(readerAt, stat.Size())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "magic footer") {
			return 0, fmt.Errorf("not a valid parquet file: %w", err)
		}
		return 0, err
	}

	return pf.NumRows(), nil
}
