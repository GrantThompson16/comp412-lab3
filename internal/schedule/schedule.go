package schedule

import (
	"fmt"
	"io"

	"lab3/internal/frontend/ir"
)

func Schedule(irList *ir.IR, w io.Writer) error {
	if irList == nil {
		return fmt.Errorf("ERROR: nil IR passed to scheduler")
	}
	if w == nil {
		return fmt.Errorf("ERROR: nil writer passed to scheduler")
	}

	// build scheduler model
	instructions := buildInstructions(irList)

	maxVR := irList.MaxVR()

	_ = buildDepGraph(instructions, maxVR)

	irList.Fprint(w)
	return nil
}

func DumpDG(irList *ir.IR, w io.Writer) error {
    instrs := buildInstructions(irList)
    maxVR := irList.MaxVR()
    g := buildDepGraph(instrs, maxVR)
	g.ComputePriorities()
    return g.FprintDOT(w)
}