package schedule

import (
	"fmt"
	"io"
	"container/heap"

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

// ===== Ready set (max heap by priority) ====
type readyPQ struct {
	g *DepGraph
	items []int //instruction indicies
}

func (pq readyPQ) Len() int {
	return len(pq.items)
}

func (pq readyPQ) Less(i, j int) bool {
    ia := pq.items[i]
    ib := pq.items[j]
    pa := pq.g.Nodes[ia].Priority
    pb := pq.g.Nodes[ib].Priority
    if pa != pb {
        // higher priority should come first
        return pa > pb
    }
    return ia < ib
}

func (pq readyPQ) Swap(i, j int) { 
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i] 
}

func (pq *readyPQ) Push(x any) {
    pq.items = append(pq.items, x.(int))
}

func (pq *readyPQ) Pop() any {
    n := len(pq.items)
    x := pq.items[n-1]
    pq.items = pq.items[:n-1]
    return x
}

// === pending set (min heap by ready cycle)
type pendingInstr struct {
	readyCycle int
	idx int
}

type pendingPQ []pendingInstr

func (pq pendingPQ) Len() int {
	return len(pq)
}

func (pq pendingPQ) Less(i, j int) bool {
    if pq[i].readyCycle != pq[j].readyCycle {
        return pq[i].readyCycle < pq[j].readyCycle
    }
    return pq[i].idx < pq[j].idx
}

func (pq pendingPQ) Swap(i,j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *pendingPQ) Push(x any) {
    *pq = append(*pq, x.(pendingInstr))
}

func (pq *pendingPQ) Pop() any {
    old := *pq
    n := len(old)
    x := old[n-1]
    *pq = old[:n-1]
    return x
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


	predsRemaining := make([]int, n) // num unscheduled preds for instruction i
	readyCycle := make([]int, n) // earliest cycle when op can issue

	for i, node := range g.Nodes {
		predsRemaining[i] = len(node.Out)
		readyCycle[i] = 0
	}


	// Ready PQ, ordered by priortity, max heap
	rpq := &readyPQ{
		g: g,
	}
	heap.Init(rpq)

	//pending PQ ordered by readyCycle (min heap)
	pending := &pendingPQ{}
	heap.Init(pending)

	// initially, any node w 0 preds becomes ready at cycle 0
	for i :=0; i < n; i++ {
		if predsRemaining[i] == 0 {
			heap.Push(rpq, i)
		}
	}
	
	remaining := n
	var schedule [][]*Instr
	cycle := 0

	for remaining > 0 {
		for pending.Len() > 0 && (*pending)[0].readyCycle <= cycle {
			item := heap.Pop(pending).(pendingInstr)
			heap.Push(rpq, item.idx)
		}

		fu0Free, fu1Free := true, true
		outputUsed := false
		var issued []*Instr

		for picks := 0; picks < 2 && rpq.Len() > 0; picks++ {
			chosenIdx := -1
			chosenFU := -1

			var skipped []int

			for rpq.Len() > 0 {
				idx := heap.Pop(rpq).(int)
				inst := g.Instructions[idx]
				fu := chooseFU(inst, fu0Free, fu1Free, outputUsed)
				if fu < 0 {
					skipped = append(skipped, idx)
					continue
				}
				chosenIdx = idx
				chosenFU = fu
				break
			}

			for _, idx := range skipped {
				heap.Push(rpq, idx)
			}

			if chosenIdx == -1 {
				break
			}

			inst := g.Instructions[chosenIdx]
			issued = append(issued, inst)
			remaining--

			if chosenFU == 0 {
				fu0Free = false
			} else {
				fu1Free = false
			}
			if inst.IsOutput {
				outputUsed = true
			}

			node := g.Nodes[chosenIdx]
			for _, e := range node.In {
				succ := e.From 

				predsRemaining[succ]--
				if predsRemaining[succ] < 0 {
					predsRemaining[succ] = 0
				}

				t := cycle + e.Latency
				if t > readyCycle[succ] {
					readyCycle[succ] = t
				}

				if predsRemaining[succ] == 0 {
					rt := readyCycle[succ]
					if rt <= cycle {
						heap.Push(rpq, succ)
					} else {
						heap.Push(pending, pendingInstr{readyCycle: rt, idx: succ})
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