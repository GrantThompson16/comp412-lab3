package rename

import (
	"lab3/internal/frontend/ir"
)

func RenameVirtualRegisters(irList *ir.IR) {
	if irList == nil || irList.Count == 0 {
		return // Empty IR list theres nothing to do
	}

	// Ensure VR/PR/NU start from clean slate
	irList.ResetAllRenameAlloc()

	// Create size for vectors based on largest SR present
	maxSR := irList.MaxSR()
	if maxSR < 0 {
		return //no registers in this IR (error or only output constants)
	}

	// Allocate slices for renamer pass
	srToVR := make([]int, maxSR+1)
	lastUse := make([]int, maxSR+1)
	//pre populate with infs/invalids
	for i := range srToVR {
		srToVR[i] = ir.InvalidReg
		lastUse[i] = ir.NUInf
	}

	nextVR := 0 // virtual register counter (v0, v1, v2, ....)

	// Walk the IR list backwards
	irList.ForEachReverse(func(node *ir.IRNode, idx int) {
		// Handle defs
		for _, def := range node.GetDefs() {
			vr := srToVR[def.SR]
			if vr == ir.InvalidReg {
				vr = nextVR
				nextVR++
			}

			def.VR = vr
			def.NU = lastUse[def.SR]

			srToVR[def.SR] = ir.InvalidReg
			lastUse[def.SR] = ir.NUInf
		}

		// Handle uses
		var usedSR [2]int
		k := 0

		for _, use := range node.GetUses() {
			vr := srToVR[use.SR]
			if vr == ir.InvalidReg {
				// no current virtual reg for this SR, use begins a live range
				vr = nextVR
				nextVR++
				srToVR[use.SR] = vr
			}

			use.VR = vr
			use.NU = lastUse[use.SR]

			usedSR[k] = use.SR
			k++
		}

		for i := 0; i < k; i++ {
			lastUse[usedSR[i]] = idx
		}
	})
}
