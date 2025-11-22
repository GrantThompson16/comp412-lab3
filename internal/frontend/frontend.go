package frontend

import (
	"fmt"
	"io"
	"os"

	"lab3/internal/frontend/ir"
	"lab3/internal/frontend/parser"
	"lab3/internal/frontend/scanner"
)

// This method turns an ILOC stream into an IR
// Returns the IR, opcount, and error (nil on success)
func parseReader(r io.Reader) (*ir.IR, int, error) {
	// reader not set / nil return error
	if r == nil {
		return nil, 0, fmt.Errorf("nil reader")
	}

	s := scanner.New(r, false, true)
	p := parser.New(s, os.Stderr)
	res := p.Parse()

	if res.HadErrors {
		return nil, res.OpCount, fmt.Errorf("Parse failed")
	}

	if res.IR == nil {
		return nil, res.OpCount, fmt.Errorf("Parser returned nil IR without errors")
	}

	return res.IR, res.OpCount, nil
}

// This method is a ParseReader wrapper for an input (cmd line inputted) file path
// This method is called from main
func ParseFile(path string) (*ir.IR, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()
	return parseReader(f)
}