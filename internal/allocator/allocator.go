package allocator

import (
	"fmt"

	"lab2/internal/frontend/ir"
	"lab2/internal/frontend/token"
)

const invalid = -1

const (
	spillBaseAddr = 32768
	spillLen      = 4
)

type allocState struct {
	k        int //total physical registers requested
	spillReg int // k-1 if reg reserved, -1 if no spill reg reserved
	usablePR int // num of PRs allowed to assign to VRs (k or k-1)

	VRToPR       []int
	PRToVR       []int
	VRToSpillLoc []int  // VR -> mem addr
	PRNU         []int  //PR -> next use idx for its occupant vr
	defIsLoadI   []bool // VR -> defined by LoadI?
	loadIConst   []int  // VR -> constant used in loadI (valid if defIsLoadI[vr])

	nextSpillAddr int
	opIndex       int //curr instruction index
}

func Allocate(irList *ir.IR, k int) (*ir.IR, error) {
	if irList == nil {
		return nil, fmt.Errorf("nil IR")
	}

	if k <= 0 {
		return nil, fmt.Errorf("k must be greater than 0")
	}

	maxVR := irList.MaxVR()
	if maxVR < 0 {
		// no registers in block, nothing to do
		return irList, nil
	}

	// ======= Init Allocator state and maps ============
	state := &allocState{
		k:             k,
		spillReg:      -1,
		usablePR:      k,
		VRToPR:        make([]int, maxVR+1),
		PRToVR:        make([]int, k),
		VRToSpillLoc:  make([]int, maxVR+1),
		PRNU:          make([]int, k),
		defIsLoadI:    make([]bool, maxVR+1),
		loadIConst:    make([]int, maxVR+1),
		nextSpillAddr: spillBaseAddr,
		opIndex:       0,
	}

	// Map init/setup
	for i := range state.VRToPR {
		state.VRToPR[i] = invalid
		state.VRToSpillLoc[i] = invalid
	}

	for i := range state.PRToVR {
		state.PRToVR[i] = invalid
		state.PRNU[i] = ir.NUInf
	}

	// prepass to record loadI rematerializables and what constant associated w/ them
	irList.ForEach(func(node *ir.IRNode) {
		if node.Opcode == token.OP_LOADI {
			defs := node.GetDefs()
			if len(defs) == 1 {
				vr := defs[0].VR
				if vr >= 0 {
					state.defIsLoadI[vr] = true
					state.loadIConst[vr] = node.Source1.SR
				}
			}
		}
	})

	// Decide whether to reserve a spill register
	maxLive := findMaxLive(irList)

	state.k = k
	if maxLive > k && k > 1 {
		// MUST spill so reserve a register
		state.spillReg = k - 1
		state.usablePR = k - 1
	} else {
		// MIGHT fit so use all registers
		state.spillReg = -1
		state.usablePR = k
	}

	for cur := irList.Head; cur != nil; {
		next := cur.Next

		if cur.Opcode == token.OP_LOADI && cur.Dest.IsRegister() && cur.Dest.VR >= 0 {
			irList.Remove(cur)
			cur = next
			continue
		}
		// Protect the PRs that the current op's uses need
		prot := initProtectedPRs()
		irUses := cur.GetUses()

		for _, u := range irUses {
			if u.IsRegister() && u.VR >= 0 {
				if pr := state.VRToPR[u.VR]; pr != invalid {
					prot.add(pr)
				}
			}
		}

		var spills []*ir.IRNode
		var restores []*ir.IRNode

		// ensure each use is in some PR at op start
		for _, u := range irUses {
			if !u.IsRegister() || u.VR < 0 {
				continue
			}

			// vr already lives in some PR
			pr := state.VRToPR[u.VR]
			if pr != invalid {
				u.PR = pr
				if pr >= 0 && pr < state.usablePR {
					state.PRNU[pr] = u.NU
					prot.add(pr)
				}
				continue
			}

			if state.spillReg >= 0 &&
				state.defIsLoadI[u.VR] &&
				prot.allProtected(state.usablePR) {
				loadI := ir.NewLoadI(state.loadIConst[u.VR], state.spillReg, cur.Line)
				setPRs(loadI)
				restores = append(restores, loadI)
				u.PR = state.spillReg
				prot.add(state.spillReg)
				continue
			}

			pr, spillSeq := state.getPRorSpill(irList, cur.Line, prot)
			if len(spillSeq) > 0 {
				spills = append(spills, spillSeq...)
			}

			// rematerialize value in 'pr' or restore from memory
			if state.defIsLoadI[u.VR] {
				// rematerialize
				loadI := ir.NewLoadI(state.loadIConst[u.VR], pr, cur.Line)
				setPRs(loadI)
				restores = append(restores, loadI)
			} else {
				// normal restore from spill slot: loadI <addr> -> temp ; load temp -> pr
				if state.spillReg < 0 {
					panic("restore requires spilReg, one not reserved")
				}
				if pr == state.spillReg {
					panic("restore chose spillReg as destPR")
				}

				addr := state.getSpillAddr(u.VR)
				loadAddr := ir.NewLoadI(addr, state.spillReg, cur.Line)
				loadDest := ir.NewLoad(state.spillReg, pr, cur.Line)
				setPRs(loadAddr)
				setPRs(loadDest)
				restores = append(restores, loadAddr, loadDest)
			}

			state.bind(u.VR, pr)
			u.PR = pr
			if pr >= 0 && pr < state.usablePR {
				state.PRNU[pr] = u.NU
				prot.add(pr)
			}
		}

		// Insert all SPILLs and then all RESTORES/rematerializations, then the original op
		if len(spills) > 0 {
			irList.InsertSeqNodesBefore(cur, spills...)
		}
		if len(restores) > 0 {
			irList.InsertSeqNodesBefore(cur, restores...)
		}

		// Free PRs whose VRs have last use at this current OP (nu = inf)
		for _, u := range irUses {
			if u.IsRegister() && u.VR >= 0 && u.NU == ir.NUInf {
				pr := state.VRToPR[u.VR]
				if pr != invalid {
					state.unbind(u.VR, pr)
				}
			}
		}
		// Allocate PR for defs
		defs := cur.GetDefs()
		if len(defs) == 1 {
			d := defs[0]
			if d.IsRegister() && d.VR >= 0 {
				pr := state.VRToPR[d.VR]
				if pr == invalid {
					if prot.allProtected(state.usablePR) {
						irUses := cur.GetUses()

						candPR, candNU := -1, -1
						candVR := -1
						isZeroCost := func(vr int) bool {
							return state.defIsLoadI[vr] || state.VRToSpillLoc[vr] != invalid
						}

						for _, u := range irUses {
							if !u.IsRegister() || u.VR < 0 {
								continue
							}
							srcPR := state.VRToPR[u.VR]
							if srcPR == invalid {
								continue
							}
							if isZeroCost(u.VR) {
								nu := state.PRNU[srcPR]
								if candPR == -1 || !isZeroCost(candVR) || nu > candNU {
									candPR, candNU, candVR = srcPR, nu, u.VR
								}
							} else if candPR == -1 || (!isZeroCost(candVR) && state.PRNU[srcPR] > candNU) {
								candPR, candNU, candVR = srcPR, state.PRNU[srcPR], u.VR
							}
						}

						if candPR == -1 {
							// Fall back to normal spill path.
							assignPR, spillSeq := state.getPRorSpill(irList, cur.Line, prot)
							if len(spillSeq) > 0 {
								irList.InsertSeqNodesBefore(cur, spillSeq...)
							}
							state.bind(d.VR, assignPR)
							pr = assignPR
						} else {
							// If candidate isn’t zero-cost spill it first
							if !isZeroCost(candVR) {
								if state.spillReg < 0 || state.spillReg == candPR {
									panic("allocator: spillReg unavailable for DEF spill")
								}
								addr := state.getSpillAddr(candVR)
								la := ir.NewLoadI(addr, state.spillReg, cur.Line)
								st := ir.NewStore(candPR, state.spillReg, cur.Line)
								setPRs(la)
								setPRs(st)
								irList.InsertSeqNodesBefore(cur, la, st) // spill before op
							} else {
								state.unbind(candVR, candPR)
							}
							state.bind(d.VR, candPR)
							pr = candPR
						}
					} else {
						// Normal path: get a PR (may spill a victim)
						assignPR, spillSeq := state.getPRorSpill(irList, cur.Line, prot)
						if len(spillSeq) > 0 {
							irList.InsertSeqNodesBefore(cur, spillSeq...)
						}
						state.bind(d.VR, assignPR)
						pr = assignPR
					}
				}
				d.PR = pr
				if pr >= 0 && pr < state.usablePR {
					state.PRNU[pr] = d.NU
				}
			}
		}
		state.opIndex++
		cur = next
	}
	return irList, nil
}

// ================= Allocation Helpers ============

func findMaxLive(irList *ir.IR) int {
	live := 0
	maxLive := 0

	irList.ForEach(func(node *ir.IRNode) {
		if live > maxLive {
			maxLive = live
		}

		lastUses := 0
		for _, use := range node.GetUses() {
			if use.IsRegister() && use.NU == ir.NUInf {
				lastUses++
			}
		}

		defCount := 0
		defs := node.GetDefs()
		if len(defs) == 1 && defs[0].IsRegister() {
			defCount = 1
		}

		live = live - lastUses + defCount

		if live > maxLive {
			maxLive = live
		}
	})

	return maxLive
}

// Obtain a physical register to use right now
func (state *allocState) getPRorSpill(irList *ir.IR, line int, prot protectedPRs) (int, []*ir.IRNode) {

	// prioritize free PRs not protected by current op
	for pr := 0; pr < state.usablePR; pr++ {
		if state.PRToVR[pr] == invalid && !prot.has(pr) {
			return pr, nil
		}
	}

	// If none of non-protected PRs free take any free PR
	// MAYEB DELETE
	for pr := 0; pr < state.usablePR; pr++ {
		if state.PRToVR[pr] == invalid {
			return pr, nil
		}
	}

	for pr := 0; pr < state.usablePR; pr++ {
		if prot.has(pr) {
			continue
		}
		if state.PRToVR[pr] != invalid && state.PRNU[pr] == ir.NUInf {
			victimVR := state.PRToVR[pr]
			state.unbind(victimVR, pr)
			return pr, nil
		}
	}

	// no free PR choose victim to spill
	victimPR := state.getVictimPR(prot)
	if prot.has(victimPR) {
		panic(fmt.Errorf("getVictimPR returned protected PR r%d", victimPR))
	}
	victimVR := state.PRToVR[victimPR]

	// victim rematerializable, spill cost of 0
	if state.defIsLoadI[victimVR] || state.VRToSpillLoc[victimVR] != invalid {
		state.unbind(victimVR, victimPR)
		if prot.has(victimPR) {
			panic(fmt.Errorf("returning protected PR r%d after unbind", victimPR))
		}
		return victimPR, nil
	}

	if state.spillReg < 0 || victimPR == state.spillReg {
		panic("getPRorSpill: spillReg unavailable or chosen as victim")
	}

	addr := state.getSpillAddr(victimVR)
	loadAddr := ir.NewLoadI(addr, state.spillReg, line)
	store := ir.NewStore(victimPR, state.spillReg, line)
	setPRs(loadAddr)
	setPRs(store)

	// After emitting spill code, free vixtimPR for reuse
	state.unbind(victimVR, victimPR)

	if prot.has(victimPR) {
		panic(fmt.Errorf("returning protected PR r%d after spill", victimPR))
	}
	// return newly freed PR and the spill sequence to insert before current op
	return victimPR, []*ir.IRNode{loadAddr, store}

}

// Chooses cheapest to spill PR first (rematerializable), else farthest use (largest PRNU), skips protected PRs
func (state *allocState) getVictimPR(prot protectedPRs) int {
	bestPR, bestNU := -1, -1

	for pr := 0; pr < state.usablePR; pr++ {
		if prot.has(pr) {
			continue
		}
		if state.PRToVR[pr] != invalid && state.PRNU[pr] == ir.NUInf {
			return pr
		}
	}

	// Pick the one w/ farthest next use
	for pr := 0; pr < state.usablePR; pr++ {
		if prot.has(pr) {
			continue
		}

		// Prioritize PRs in loadI's
		vr := state.PRToVR[pr]
		if vr == invalid || !state.defIsLoadI[vr] {
			continue
		}
		nu := state.PRNU[pr]
		if nu > bestNU {
			bestNU, bestPR = nu, pr
		}
	}

	if bestPR != -1 {
		return bestPR
	}

	// // already spilled victims (no store)
	bestPR, bestNU = -1, -1
	for pr := 0; pr < state.usablePR; pr++ {
		if prot.has(pr) {
			continue
		}

		vr := state.PRToVR[pr]
		if vr == invalid || state.defIsLoadI[vr] {
			continue
		}

		if state.VRToSpillLoc[vr] == invalid {
			continue
		}

		nu := state.PRNU[pr]
		if nu > bestNU {
			bestNU, bestPR = nu, pr
		}
	}

	// No rematerializable, just pick farthest next use
	bestPR, bestNU = -1, -1
	for pr := 0; pr < state.usablePR; pr++ {
		if prot.has(pr) {
			continue
		}

		nu := state.PRNU[pr]
		if nu > bestNU {
			bestNU, bestPR = nu, pr
		}
	}

	return bestPR
}

func (s *allocState) bind(vr, pr int) {
	if vr == invalid || pr == invalid {
		panic("bind: invalid vr/pr")
	}

	// If this PR has a different occupant, evict it.
	if occ := s.PRToVR[pr]; occ != invalid && occ != vr {
		s.VRToPR[occ] = invalid
		// also clear PR side for the old PR->VR relation
	}

	// If this VR is currently in some other PR, clear that PR.
	if old := s.VRToPR[vr]; old != invalid && old != pr {
		s.PRToVR[old] = invalid
		if old >= 0 && old < len(s.PRNU) {
			s.PRNU[old] = ir.NUInf
		}
	}

	s.VRToPR[vr] = pr
	s.PRToVR[pr] = vr
}

func (state *allocState) unbind(vr int, pr int) {
	if pr >= 0 {
		state.PRToVR[pr] = invalid
		state.PRNU[pr] = ir.NUInf
	}

	if vr >= 0 {
		state.VRToPR[vr] = invalid
	}
}

func (state *allocState) getSpillAddr(vr int) int {
	if state.VRToSpillLoc[vr] != invalid {
		return state.VRToSpillLoc[vr] //already has a slot, reuse it
	}

	addr := state.nextSpillAddr
	state.nextSpillAddr += spillLen
	state.VRToSpillLoc[vr] = addr
	return addr
}

// ================ Protect Physical registers already used in current operation (at most 2) ======================
type protectedPRs struct {
	a int
	b int
}

func initProtectedPRs() protectedPRs {
	return protectedPRs{a: -1, b: -1}
}

func (p protectedPRs) has(pr int) bool {
	return pr >= 0 && (pr == p.a || pr == p.b)
}

func (p *protectedPRs) add(pr int) {
	if pr < 0 {
		return
	}

	if p.a == -1 || p.a == pr {
		p.a = pr
		return
	}

	if p.b == -1 || p.b == pr {
		p.b = pr
		return
	}
}

func (p protectedPRs) allProtected(usablePR int) bool {
	switch usablePR {
	case 0:
		return true
	case 1:
		return p.a != -1 // the single PR is protected
	default: // usablePR >= 2; we only track up to 2 protected (max 2 uses)
		return usablePR == 2 && p.a != -1 && p.b != -1 && p.a != p.b
	}
}

// For allocator created bodes, set SR to PR
func setPRs(node *ir.IRNode) {
	if node.Source1.IsRegister() {
		node.Source1.PR = node.Source1.SR
	}
	if node.Source2.IsRegister() {
		node.Source2.PR = node.Source2.SR
	}
	if node.Dest.IsRegister() {
		node.Dest.PR = node.Dest.SR
	}

}
