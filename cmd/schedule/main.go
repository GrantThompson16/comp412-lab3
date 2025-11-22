package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"lab3/internal/frontend"
	"lab3/internal/rename"
	"lab3/internal/schedule"
)

func main() {
	// flags for -h and -x
	helpFlag := flag.Bool("h", false, "prints usage and exits")
	renamePrintFlag := flag.String("x", "", "scans, parses, renames, prints renamed IR (debug)")
	dgFlag := flag.String("dg", "", "dump dependence graph for <iloc file> (debug)")
	flag.Parse()

	if *helpFlag {
		fmt.Fprint(os.Stdout, usage())
		return
	}

	// Debug mode: schedule -x <file>
	if *renamePrintFlag != "" {
		err := runRename(*renamePrintFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	// Debug mode: schedule -dg <file>
	if *dgFlag != "" {
		if err := runDumpDG(*dgFlag); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	// non-flag arg (ex[ect exactly one <file> to schedule)
	args := flag.Args()
	if len(args) == 1 {
		path := args[0]
		err := runSchedule(path)
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
	%s <iloc file>

	Flags:
	-h:		  		Prints this help/usage message and exit.
	-x <file>		(USED FOR DEBUGGING) Scans/parses <file> and build an IR. Renames the Source Registers to Virtual Registers and prints the IR in a readable format.		
	
	Modes:
	%s <file>		Scans/parses <file>, renames SRs to VRs, and runs the scheduler on the resulting basic block. The scheduled ILOC is printed to stdout.
	

	Notes:
	- These flags are mutually exclusive. They are prioritized in order of -h, -x.
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
		return fmt.Errorf("ERROR: printing renamed IR failed: %w", err)
	}
	return nil
}

func runSchedule(path string) error {
	irList, _, err := frontend.ParseFile(path)
	if err != nil {
		return fmt.Errorf("ERROR: Parse failed for %q: %w", path, err)
	}

	// Rename before scheduling
	rename.RenameVirtualRegisters(irList)

	err = schedule.Schedule(irList, os.Stdout)
	if err != nil {
		return fmt.Errorf("ERROR: scheduling failed: %w", err)
	}
	return nil
}

func runDumpDG(path string) error {
	irList, _, err := frontend.ParseFile(path)
	if err != nil {
		return fmt.Errorf("ERROR: parse failed for %q: %w", path, err)
	}

	rename.RenameVirtualRegisters(irList)

	if err := schedule.DumpDG(irList, os.Stdout); err != nil {
		return fmt.Errorf("ERROR: dumping dependence graph failed: %w", err)
	}
	return nil
}