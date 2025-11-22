package schedule

import (
	"lab3/internal/frontend/ir"
	"lab3/internal/frontend/token"
)

/**
========= Model.go provides a way to build a more simple version of the IR, 
allowing cheap indexing as it is a slice and not linked list ===========
**/



// Instr represents one schedulable instruction
type Instr struct {
	Index int // index in instr slice
	Node *ir.IRNode // uderlying ir node
	Opcode token.Operation // type of operation
	Latency int //cycles

	// Register uses and defs in terms of VRs from the renamer
	SrcVR []int // slice of source VRs (0 ,1 or 2)
	DestVR int // -1 if no dest VR

	// Memory / output classification
	IsLoad bool
	IsStore bool
	IsOutput bool
}

// latency table
func latencyForOpcode(op token.Operation) int {
	switch op {
	case token.OP_LOAD, token.OP_STORE:
		return 6
	case token.OP_MULT:
		return 3
	default:
		return 1
	}
}

func buildInstructions(irList *ir.IR) []*Instr{
	if irList == nil || irList.Count == 0 {
		return nil
	}

	var instructions []*Instr
	index := 0

	for node := irList.Head; node != nil; node = node.Next {
		// skip inputs nops, they have no effect
		if node.Opcode == token.OP_NOP{
			continue
		}

		inst := &Instr{
			Index: index,
			Node: node,
			Opcode: node.Opcode,
			Latency: latencyForOpcode(node.Opcode),
			SrcVR: make([]int, 0, 2),
			DestVR: ir.InvalidReg,
		}

		// Uses
		for _, opnd := range node.GetUses() {
			if opnd.IsRegister() && opnd.VR != ir.InvalidReg {
				inst.SrcVR = append(inst.SrcVR, opnd.VR)
			}
		}

		// defs
		defs := node.GetDefs()
		if len(defs) > 0 && defs[0].IsRegister() && defs[0].VR != ir.InvalidReg {
			inst.DestVR = defs[0].VR
		} else {
			inst.DestVR = ir.InvalidReg
		}

		// output classification
		switch node.Opcode {
		case token.OP_LOAD:
			inst.IsLoad = true
		case token.OP_STORE:
			inst.IsStore = true
		case token.OP_OUTPUT:
			inst.IsOutput = true
		}

		instructions = append(instructions, inst)
		index++
	}
	return instructions
}