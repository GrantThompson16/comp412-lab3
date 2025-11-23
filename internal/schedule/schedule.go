package schedule

import (
	"fmt"
	"io"
	"sort"

	"lab3/internal/frontend/ir"
	"lab3/internal/frontend/token"
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
	if len(instructions) == 0 {
		return nil //maybe err
	}

	maxVR := irList.MaxVR()
	g := buildDepGraph(instructions, maxVR)
	g.ComputePriorities()

	cycles := scheduleBlock(g)

	for _, ops := range cycles {
		switch len(ops) {
		case 0:
			// stall cycle
			fmt.Fprintln(w, "[ nop ]")
		case 1:
			s1, err := formatOp(ops[0].Node)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "[ %s ]\n", s1)
		case 2:
			s1, err1 := formatOp(ops[0].Node)
			if err1 != nil {
				return err1
			}
			s2, err2 := formatOp(ops[1].Node)
			if err2 != nil {
				return err2
			}
			fmt.Fprintf(w, "[ %s ; %s ]\n", s1, s2)
		default:
			//should never happen, at most 2 ops per cycle
			return fmt.Errorf("ERROR: more than 2 ops in a cycle")
		}
	}
	return nil
}

// decides which functiopnal unit (0 or 1) this instruction can run this cycle
// given current FU availability and whether an output alr been issued this cuycle.
// Returns -1 if it cannot be scheduled this cycle
func chooseFU(inst *Instr, fu0, fu1, outputUsed bool) int {
	op := inst.Opcode

	switch op {
	case token.OP_LOAD, token.OP_STORE:
		if fu0 {
			return 0
		}
		return -1
	case token.OP_MULT:
		//mult must run on f1
		if fu1{
			return 1
		}
		return -1
	case token.OP_OUTPUT:
		if outputUsed{
			return -1
		}
		if fu1 {
			return 1
		}
		if fu0 {
			return 0
		}
		return -1
	default:
		if fu1 {
			return 1
		}
		if fu0 {
			return 0
		}
		return -1
	}
}

// Performs the list scheduling on dependence graph g
// returns slice of cycles, each cycle is a slice of up to 2 *Instr
// empty slices represent stall cycles (nops)
func scheduleBlock(g *DepGraph) [][]*Instr {
	n := len(g.Instructions)
	if n == 0 {
		return nil
	}

	unscheduledPreds := make([]int, n) // num predecessors (earlier depoendencies) remain
	readyCycle := make([]int, n) // earliest cycle when op can issue
	scheduled := make([]bool, n) // has op been scheduled?
	remaining := n

	for i, node := range g.Nodes {
		unscheduledPreds[i] = len(node.Out)
	}

	var schedule [][]*Instr
	cycle := 0

	for remaining > 0 {
		// collect all currently ready ops at this cycle
		var ready []int
		for i := 0; i < n; i++ {
			if scheduled[i] {
				continue
			}
			if unscheduledPreds[i] == 0 && readyCycle[i] <= cycle {
				ready = append(ready, i)
			}
		}

		fu0Free, fu1Free := true, true
		outputUsed := false
		var issued []*Instr

		if len(ready) > 0 {
			// sort ready by priority high to low, then index low to high
			sort.SliceStable(ready, func(a,b int) bool {
				ia := ready[a]
				ib := ready[b]
				pa := g.Nodes[ia].Priority
				pb := g.Nodes[ib].Priority
				if pa != pb {
					return pa > pb
				}
				return ia < ib
			})

			used := make([]bool, len(ready))

			// try to pick up 2 ops for this cycle
			for picks := 0; picks < 2; picks++ {
				bestIdx := -1
				bestFU := -1

				for pos, idx := range ready {
					if used[pos] {
						continue
					}
					inst := g.Instructions[idx]
					fu := chooseFU(inst, fu0Free, fu1Free, outputUsed)
					if fu < 0 {
						continue
					}

					bestIdx = idx
					bestFU = fu
					used[pos] = true
					break
				
				}
				if bestIdx == -1 {
					//no more ops fit this cycle 
					break
				}

				inst := g.Instructions[bestIdx]
				issued = append(issued, inst)
				scheduled[bestIdx] = true
				remaining--

				// consume chosen fu and output slot if needed
				if bestFU == 0 {
					fu0Free = false
				} else if bestFU == 1 {
					fu1Free = false
				}
				if inst.IsOutput {
					outputUsed = true
				}

				// update successors (later ops) that depend on this op
				node := g.Nodes[bestIdx]
				for _, e := range node.In {
					succ := e.From
					if scheduled[succ] {
						continue
					}

					unscheduledPreds[succ]--
					if unscheduledPreds[succ] < 0 {
						unscheduledPreds[succ] = 0
					}

					rc := cycle + e.Latency
					if rc > readyCycle[succ] {
						readyCycle[succ] = rc
					}
				}
			}
		}

		schedule = append(schedule, issued)
		cycle++
	}
	return schedule

}









// ===== Printing Helpers ===========
func formatRegVR(op ir.Operand) string {
	if op.VR >= 0 {
		return fmt.Sprintf("r%d", op.VR)
	}
	if op.Present && !op.IsConst {
		return fmt.Sprintf("r%d", op.SR)
	}
	return ""
}

func formatOp(node *ir.IRNode) (string, error) {
	switch node.Opcode {
	case token.OP_LOAD:
		return fmt.Sprintf("load %s => %s",
			formatRegVR(node.Source1), formatRegVR(node.Dest)), nil

	case token.OP_STORE:
		return fmt.Sprintf("store %s => %s",
			formatRegVR(node.Source1), formatRegVR(node.Dest)), nil

	case token.OP_LOADI:
		return fmt.Sprintf("loadI %d => %s",
			node.Source1.SR, formatRegVR(node.Dest)), nil

	case token.OP_ADD, token.OP_SUB, token.OP_MULT,
		token.OP_LSHIFT, token.OP_RSHIFT:
		return fmt.Sprintf("%s %s, %s => %s",
			node.Opcode.String(),
			formatRegVR(node.Source1),
			formatRegVR(node.Source2),
			formatRegVR(node.Dest)), nil

	case token.OP_OUTPUT:
		return fmt.Sprintf("output %d", node.Source1.SR), nil

	case token.OP_NOP:
		return "nop", nil

	default:
		return "", fmt.Errorf("ERROR: unsupported opcode %q on line %d",
			node.Opcode.String(), node.Line)
	}
}

// ===== Debug Helper ==========
func DumpDG(irList *ir.IR, w io.Writer) error {
    instrs := buildInstructions(irList)
    maxVR := irList.MaxVR()
    g := buildDepGraph(instrs, maxVR)
	g.ComputePriorities()
    return g.FprintDOT(w)
}