package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"lab2/internal/frontend"
	"lab2/internal/rename"
	"lab2/internal/allocator"
)

func main() {
	// flags for -h and -x
	helpFlag := flag.Bool("h", false, "prints usage and exits")
	renamePrintFlag := flag.String("x", "", "scans, parses, renames, prints")
	flag.Parse()

	if *helpFlag {
		fmt.Fprint(os.Stdout, usage())
		return
	}

	if *renamePrintFlag != "" {
		err := runRename(*renamePrintFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	// non-flag arg (k <name>)
	args := flag.Args()
	if len(args) == 2 {
		kStr, path := args[0], args[1]
		k, err := strconv.Atoi(kStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Invalid k %q: %v\n", kStr, err)
			os.Exit(1)
		}

		if k < 3 || k > 64 {
			fmt.Fprintf(os.Stderr, "ERROR: k out of range: %d (expected 3...64)\n", k)
			os.Exit(1)
		}

		err = runAllocate(k, path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stderr, "ERROR: Invalid args")
	fmt.Fprint(os.Stdout, usage())
	os.Exit(2)
}

func usage() string {
	prog := filepath.Base(os.Args[0])
	return fmt.Sprintf(`Usage:
	%s -h
	%s -x <iloc file> 
	%s k <iloc file>

	Flags:
	-h:		  		Prints this help/usage message and exit.
	-x <file>		Scans/parses <file> and build an IR. Renames the Source Registers to Virtual Registers and prints the IR in a readable format.		
	k <file>		Run the allocator with k physical registers (3 <= k <= 64) on <file>, print allocated ILOC.
	

	Notes:
	- These flags are mutually exclusive. They are prioritized in order of -h, -x, k.
	- Non-error output is printed to Stdout. Error messages are pritned to stderror and are formatted as 'ERROR: <line>: <error message>'\n`, prog, prog, prog)
}

// =============== Rename / Allocator Wrappers ===================
func runRename(path string) error {
	irList, _, err := frontend.ParseFile(path)
	if err != nil {
		return fmt.Errorf("ERROR: Parse failed for %q: %w", path, err)
	}

	rename.RenameVirtualRegisters(irList)

	err = irList.FprintRenamed(os.Stdout)

	if err != nil {
		return err
	}

	return nil
}

func runAllocate(k int, path string) error {
	irList, _, err := frontend.ParseFile(path)

	if err != nil {
		return fmt.Errorf("ERROR: Parse failed for %q: %w", path, err)
	}

	rename.RenameVirtualRegisters(irList)
	irList, err = allocator.Allocate(irList, k)
	if err != nil {
		return fmt.Errorf("ERROR: Allocate failed: %w", err)
	}
	
	if err := irList.FprintAllocated(os.Stdout); err != nil {
		return err
	}
	return nil
}