package schedule

// type to distinguish the diff dependency types for edges
type EdgeKind int

const (
	EdgeKindData = iota // vr
	EdgeKindConflict
	EdgeKindSerial
)


// Directed edge in the dependence graph, edges run from USE to the DEF / constraining operation
type DepEdge struct {
	From int // source instr index (use)
	To int // sink instr index (def)
	Latency int // latency of constrAINING OP
	Kind EdgeKind //kind of dependence
	VR int // vr carried on data edges, -1 for non data edges
}

// DepNode holds adjacency lists for an instruction
type DepNode struct {
	Index int
	Instr *Instr
	Out []*DepEdge
	In []*DepEdge
}

type DepGraph struct {
	Instructions []*Instr
	Nodes []*DepNode
	Edges []*DepEdge
}

// Dependency graph constructor
func newDepGraph(instructions []*Instr) *DepGraph {
	g := &DepGraph{
		Instructions: instructions,
		Nodes: make([]*DepNode, len(instructions)),
		Edges: make([]*DepEdge, 0),
	}

	for idx, inst := range instructions {
		g.Nodes[idx] = &DepNode{
			Index: idx,
			Instr: inst,
			Out: nil,
			In: nil,
		}
	}
	return g
}

// inserts a directed edge into the graph g
func (g *DepGraph) addEdge(from, to, latency int, kind EdgeKind, vr int) {
	if from == to || from < 0 || to < 0 || from >= len(g.Nodes) || to >= len(g.Nodes){
		return
	}

	e := &DepEdge{
		From: from,
		To: to,
		Latency: latency,
		Kind: kind,
		VR: vr,
	}
	g.Edges = append(g.Edges, e)
	g.Nodes[from].Out = append(g.Nodes[from].Out, e)
	g.Nodes[to].In = append(g.Nodes[to].In, e)
}

// Constructs the dependence graph
// 1. For each VR use, add a data edge from use to its definition
// 2. for memort ops (load store output) add CONFLICT and SERIAL edges 
// edges always run from use to the earlier op (def)
func buildDepGraph(instructions []*Instr, maxVR int) *DepGraph {
	g := newDepGraph(instructions)
	n := len(instructions)
	if n == 0 {
		return g
	}

	// VR data flow edges
	// For each use of VR v in instruction i, add edge i -> def(v)
	if maxVR >= 0 {
		lastDef := make([]int, maxVR+1)
		for i := range lastDef {
			lastDef[i] = -1
		}

		for i, inst := range instructions {
			for _, vr := range inst.SrcVR {
				if vr < 0 || vr > maxVR {
					continue
				}
				defIdx := lastDef[vr]
				if defIdx != -1 {
					defInst := instructions[defIdx]
					g.addEdge(i, defIdx, defInst.Latency, EdgeKindData, vr)
				}
			}
			if inst.DestVR >= 0 && inst.DestVR <= maxVR {
				lastDef[inst.DestVR] = i
			}
		}
	}

	const serialLatency = 1
	lastStore := -1
	lastOutput := -1
	sinceLastStore := []int{}

	for i, inst := range instructions {
		isLoad := inst.IsLoad
		isStore := inst.IsStore
		isOutput := inst.IsOutput

		if isLoad || isOutput {
			if lastStore != -1 {
				storeInst := instructions[lastStore]
				g.addEdge(i, lastStore, storeInst.Latency, EdgeKindConflict, -1)
			}
			sinceLastStore = append(sinceLastStore, i)
		}

		if isOutput {
			if lastOutput != -1 {
				g.addEdge(i, lastOutput, serialLatency, EdgeKindSerial, -1)
			}
			lastOutput = i
		}

		if isStore {
			if lastStore != -1 {
				g.addEdge(i, lastStore, serialLatency, EdgeKindSerial, -1)
			}
			for _, idx := range sinceLastStore {
				g.addEdge(i, idx, serialLatency, EdgeKindSerial, -1)
			}
			lastStore = i
			sinceLastStore = sinceLastStore[:0]
		}
	}
	return g
}