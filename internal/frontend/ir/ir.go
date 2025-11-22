package ir

import (
	"fmt"
	"io"
	"math"

	"lab3/internal/frontend/token"
)

const (
	InvalidReg = -1
	NUInf = math.MaxInt32
)

// Linked List container Struct for IR
type IR struct {
	Head  *IRNode // First instruciton in the list - nil when empty
	Tail  *IRNode // Last instruction in the listt - nil when empty
	Count int     // Number of Ops appended
}

// Linked List Node for IR (One per line i.e. load r1 => r2)
type IRNode struct {
	Opcode token.Operation // same enum used by scanner/parser
	Line   int             //source-line (1 based)

	//Doubly linked-list pointers for the IR representation
	Prev *IRNode
	Next *IRNode

	// Operands (at most two sources and 1 destination like add)
	Source1 Operand
	Source2 Operand
	Dest    Operand
}

// Internal structure for each operand for instance r1, or some integer constant
type Operand struct {
	Present bool // true if this operand/node exists for the opcode
	IsConst bool // true if SR stores a constant (loadI for instance)

	SR int // Source register id OR the const value when IsConst (like loadI 16)
	VR int // Virtual Register -- FILL IN FOR lab2/3 ???
	PR int // Physical Register -- FILL IN FOR LAB2/3???
	NU int // Next Use info? FILL IN FOR LAB 2/3???
}

/*
Constructor for IR linked list container
*/
func NewIr() *IR {
	return &IR{}
}

/*
Constructor for IR Linked List node
*/
func NewIRNode(op token.Operation, line int) *IRNode {
	return &IRNode{
		Opcode: op,
		Line:   line,
	}
}

/*
Append a node at the end of the IR (linked list)
O(1)
*/
func (l *IR) Append(newNode *IRNode) {
	if newNode == nil {
		return
	}
	if l.Head == nil {
		l.Head = newNode
		l.Tail = newNode
	} else {
		newNode.Prev = l.Tail
		l.Tail.Next = newNode
		l.Tail = newNode
	}
	l.Count++
}

func (node *IRNode) SetSource1(sr int) {
	node.Source1 = Operand{
		Present: true,
		SR:      sr,
	}
}

func (node *IRNode) SetSource2(sr int) {
	node.Source2 = Operand{
		Present: true,
		SR:      sr,
	}
}

func (node *IRNode) SetDest(sr int) {
	node.Dest = Operand{
		Present: true,
		SR:      sr,
	}
}

func (node *IRNode) SetConst(val int) {
	node.Source1 = Operand{
		Present: true,
		IsConst: true,
		SR:      val,
	}
}

/*
IR Helpers for Printing/Outputting (-r case)
*/

func (irList *IR) Fprint(w io.Writer) {
	if irList == nil || w == nil {
		return
	}

	for node := irList.Head; node != nil; node = node.Next {
		opname := node.Opcode.String()         // 'loadI' 'add' etc
		arg1, arg2, arg3 := formatIRArgs(node) // formats the args into brackets [ ]
		fmt.Fprintf(w, "%-7s %s, %s, %s\n", opname, arg1, arg2, arg3)
	}
}

// Helper to format the IR output
func formatIRArgs(node *IRNode) (string, string, string) {
	empty := "[ ]"
	switch node.Opcode {
	case token.OP_LOAD, token.OP_STORE, token.OP_LOADI:
		// load/store r1 => r2 is [ r1 ], [ ], [ r2 ]
		// loadI 3 => r2
		return fmtSlot(node.Source1), empty, fmtSlot(node.Dest)
	case token.OP_ADD, token.OP_SUB, token.OP_MULT, token.OP_RSHIFT, token.OP_LSHIFT:
		// add r1, r2 => r3 is [ r1 ], [ r2 ], [ r3 ]
		return fmtSlot(node.Source1), fmtSlot(node.Source2), fmtSlot(node.Dest)
	case token.OP_OUTPUT:
		// output 7 is [ val 7 ], [ ], [ ]
		return fmtSlot(node.Source1), empty, empty
	default:
		return fmtSlot(node.Source1), fmtSlot(node.Source2), fmtSlot(node.Dest)
	}
}

// Helper to craft the 'slot' string for a single IR arg
// present const --> [ val 27]
// present register --> [ sr2 ]
// no present register --> [ ]
func fmtSlot(op Operand) string {
	if !op.Present {
		return "[ ]"
	}

	if op.IsConst {
		return fmt.Sprintf("[ val %d ]", op.SR)
	}

	return fmt.Sprintf("[ sr%d ]", op.SR)
}


// ================== Lab 2 Operand Helpers =================
func (op *Operand) IsRegister() bool {
	return op.Present && !op.IsConst
}

func (op *Operand) ResetRenameAlloc() {
	op.VR, op.PR, op.NU = InvalidReg, InvalidReg, NUInf
}

// Helper to "clear" the 
func (irList *IR) ResetAllRenameAlloc() {
	for node := irList.Head; node != nil; node = node.Next {
		if node.Source1.Present {
			node.Source1.ResetRenameAlloc()
		}
		if node.Source2.Present {
			node.Source2.ResetRenameAlloc()
		}
		if node.Dest.Present {
			node.Dest.ResetRenameAlloc()
		}
	}
}


func (n *IRNode) GetDefs() []*Operand {
	switch n.Opcode{
	// All define a register with a value. Ex: load r1 => r2, defines register r2 with the bytes in memory stored at the memory address in r1
	// Ex2: add r1, r2 => r3 defines register r3 with the sum of values stored in r1 and r2
	case token.OP_ADD, token.OP_SUB, token.OP_MULT, token.OP_RSHIFT, token.OP_LSHIFT, token.OP_LOAD, token.OP_LOADI:
		if n.Dest.IsRegister(){
			return []*Operand{&n.Dest}
		}
		return nil
	case token.OP_STORE, token.OP_OUTPUT:
		return nil // no register is defined here, only used
	default:
		if n.Dest.IsRegister() {
			return []*Operand{&n.Dest}
		}
		return nil
	}
}

func (n *IRNode) GetUses() []*Operand {
	var uses []*Operand
	
	switch n.Opcode{
	case token.OP_LOAD:
		if n.Source1.IsRegister() {
			uses = append(uses, &n.Source1)
		}
	case token.OP_STORE:
		if n.Source1.IsRegister() {
			uses = append(uses, &n.Source1)
		}

		if n.Dest.IsRegister() {
			uses = append(uses, &n.Dest)
		}
	case token.OP_OUTPUT:
		// skip, const only no uses of a register. Ex: output x (where x is a const and not a register)
	default: // OP_ADD, OP_SUB, OP_MULT, OP_RSHIFT, OP_LSHIFT
		if n.Source1.IsRegister() {
			uses = append(uses, &n.Source1)
		}

		if n.Source2.IsRegister() {
			uses = append(uses, &n.Source2)
		}
	}
	return uses
}

func (irList *IR) MaxSR() int {
	max := -1
	for node := irList.Head; node != nil; node = node.Next {
		if node.Source1.IsRegister() && node.Source1.SR > max {
			max = node.Source1.SR
		}
		
		if node.Source2.IsRegister() && node.Source2.SR > max {
			max = node.Source2.SR
		}

		if node.Dest.IsRegister() && node.Dest.SR > max {
			max = node.Dest.SR
		}
	}

	return max
}

func (irList *IR) MaxVR() int {
	max := -1
	for node := irList.Head; node != nil; node = node.Next {
		if node.Source1.IsRegister() && node.Source1.VR > max {
			max = node.Source1.VR
		}
		
		if node.Source2.IsRegister() && node.Source2.VR > max {
			max = node.Source2.VR
		}

		if node.Dest.IsRegister() && node.Dest.VR > max {
			max = node.Dest.VR
		}
	}

	return max
}

func (irList *IR) ForEach(fn func(node *IRNode)) {
	idx := 0
	for node := irList.Head; node != nil; node = node.Next {
		fn(node)
		idx++
	}
}


func (irList *IR) ForEachReverse(fn func(node *IRNode, index int)) {
	idx := irList.Count - 1
	for node := irList.Tail; node != nil; node, idx = node.Prev, idx-1 {
		fn(node, idx)
	}
}

func (irList *IR) InsertBefore(at *IRNode, node *IRNode) {
	if irList == nil || node == nil {
		return
	}

	// if the list is empoty, node becomes only node
	if irList.Head == nil {
		irList.Head = node
		irList.Tail = node
		node.Next = nil
		node.Prev = nil
		irList.Count = 1
		return
	}

	// if "at" is nil, append node to end
	if at == nil {
		node.Next = nil
		node.Prev = irList.Tail

		if irList.Tail != nil {
			irList.Tail.Next = node
		}

		irList.Tail = node

		if irList.Head == nil {
			irList.Head = node
		}

		irList.Count++
		return
	}

	prev := at.Prev
	node.Prev = prev
	node.Next = at
	at.Prev = node

	if prev != nil {
		prev.Next = node
	} else { //at was the head
		irList.Head = node
	}
	irList.Count++
}

// Helper to insert a sequence of nodes in order before the node at "at", can pass 0 or more nodes
func (irList *IR) InsertSeqNodesBefore(at *IRNode, nodeSeq ...*IRNode) {
	for _, node := range nodeSeq {
		if node == nil {
			continue
		}
		irList.InsertBefore(at, node)
	}
}

func (irList *IR) Remove(node *IRNode) *IRNode {
	if irList == nil || node == nil {
		return nil
	}

	next := node.Next
	prev := node.Prev

	// Bridge neighbors
	if prev != nil {
		prev.Next = next
	} else {
		irList.Head = next
	}
	if next != nil {
		next.Prev = prev
	} else {
		irList.Tail = prev
	}

	// Fully detach node
	node.Next = nil
	node.Prev = nil

	if irList.Count > 0 {
		irList.Count--
	}
	return next
}






// ====================== Virtual Reg Rename Printers =================================
func (irList *IR) FprintRenamed(w io.Writer) error {
	if irList == nil || w == nil {
		return nil
	}

	for node := irList.Head; node != nil; node = node.Next {
		switch node.Opcode {
		case token.OP_LOAD:
			fmt.Fprintf(w, "load %s => %s\n", regVR(node.Source1), regVR(node.Dest))
		case token.OP_STORE:
			fmt.Fprintf(w, "store %s => %s\n", regVR(node.Source1), regVR(node.Dest))
		case token.OP_LOADI:
			fmt.Fprintf(w, "loadI %d => %s\n", node.Source1.SR, regVR(node.Dest))
		case token.OP_ADD, token.OP_SUB, token.OP_MULT, token.OP_LSHIFT, token.OP_RSHIFT:
			fmt.Fprintf(w, "%s %s, %s => %s\n", node.Opcode.String(), regVR(node.Source1), regVR(node.Source2), regVR(node.Dest))
		case token.OP_OUTPUT:
			fmt.Fprintf(w, "output %d\n", node.Source1.SR)
		case token.OP_NOP:
			fmt.Fprintf(w, "nop\n")
		default:
			return fmt.Errorf("line %d: unsupported opcode %q", node.Line, node.Opcode.String())
		}
	}
	return nil
}

func regVR(op Operand) string {
	if op.VR >= 0 {
		return fmt.Sprintf("r%d", op.VR)
	}

	// fail safe but should not fire after renaming
	if op.Present && !op.IsConst {
		return fmt.Sprintf("r%d", op.SR)
	}

	return ""
}

// ============================== Physical Reg (allocated) printers =================
func (irList *IR) FprintAllocated(w io.Writer) error {
	if irList == nil || w == nil {
		return nil
	}

	for node := irList.Head; node != nil; node = node.Next {
		if node.Opcode == token.OP_NOP {
			continue
		}

		switch node.Opcode {
		case token.OP_LOAD:
			fmt.Fprintf(w, "load %s => %s\n", regPR(node.Source1), regPR(node.Dest))
		case token.OP_STORE:
			fmt.Fprintf(w, "store %s => %s\n", regPR(node.Source1), regPR(node.Dest))
		case token.OP_LOADI:
			fmt.Fprintf(w, "loadI %d => %s\n", node.Source1.SR, regPR(node.Dest))
		case token.OP_ADD, token.OP_SUB, token.OP_MULT, token.OP_LSHIFT, token.OP_RSHIFT:
			fmt.Fprintf(w, "%s %s, %s => %s\n", node.Opcode.String(), regPR(node.Source1), regPR(node.Source2), regPR(node.Dest))
		case token.OP_OUTPUT:
			fmt.Fprintf(w, "output %d\n", node.Source1.SR)
		default:
			return fmt.Errorf("line %d: unsupported opcode %q", node.Line, node.Opcode.String())
		}
	}
	return nil

}

func regPR(op Operand) string {
	if op.Present && !op.IsConst {
		if op.PR >= 0 {
			return fmt.Sprintf("r%d", op.PR)
		}
		return fmt.Sprintf("sr%d", op.SR)
	}
	return ""
}

// ================= Constructors for allocator created code chunks ========================

// NewLoadI builds: loadI <const> => r<destPR>
func NewLoadI(addrConst int, destPR int, line int) *IRNode {
	node := NewIRNode(token.OP_LOADI, line)
	node.SetConst(addrConst)
	node.SetDest(destPR)
	return node
}

// NewLoad builds: load r<addrPR> => r<destPR>
func NewLoad(sourcePR int, destPR int, line int) *IRNode {
	node := NewIRNode(token.OP_LOAD, line)
	node.SetSource1(sourcePR)
	node.SetDest(destPR)
	return node
}

// NewStore builds: store r<sourcePR> => r<destPR>
func NewStore(sourcePR int, destPR, line int) *IRNode {
	node := NewIRNode(token.OP_STORE, line)
	node.SetSource1(sourcePR)
	node.SetDest(destPR)
	return node
}

