package schedule

import (
	"fmt"
	"io"

	"lab2/internal/frontend/ir"
)

func Schedule(irList *ir.IR, w io.Writer) error {
	if irList == nil {
		return fmt.Errorf("ERROR: nil IR passed to scheduler")
	}
	if w == nil {
		return fmt.Errorf("ERROR: nil writer passed to scheduler")
	}

	irList.Fprint(w)
	return nil
}