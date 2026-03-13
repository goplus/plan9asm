package plan9asm

import "fmt"

func (c *armCtx) fpResultSlotByOffset(off int64) (FrameSlot, bool) {
	for _, s := range c.fpResults {
		if s.Offset == off {
			return s, true
		}
	}
	return FrameSlot{}, false
}

func (c *armCtx) markFPResultWritten(off int64) {
	if s, ok := c.fpResultSlotByOffset(off); ok {
		c.fpResWritten[s.Index] = true
	}
}

func (c *armCtx) markFPResultAddrTaken(off int64) {
	if s, ok := c.fpResultSlotByOffset(off); ok {
		c.fpResAddrTaken[s.Index] = true
	}
}

func (c *armCtx) storeFPResult32(off int64, v32 string) error {
	slot, ok := c.fpResAllocaOff[off]
	if !ok {
		return fmt.Errorf("arm: unsupported FP result slot +%d(FP)", off)
	}
	meta, found := c.fpResultSlotByOffset(off)
	if !found {
		return fmt.Errorf("arm: missing FP result metadata for +%d(FP)", off)
	}
	switch meta.Type {
	case I32:
		fmt.Fprintf(c.b, "  store i32 %s, ptr %s\n", v32, slot)
	case I16, I8, I1:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = trunc i32 %s to %s\n", t, v32, meta.Type)
		fmt.Fprintf(c.b, "  store %s %%%s, ptr %s\n", meta.Type, t, slot)
	case Ptr:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = inttoptr i32 %s to ptr\n", t, v32)
		fmt.Fprintf(c.b, "  store ptr %%%s, ptr %s\n", t, slot)
	case I64:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i32 %s to i64\n", t, v32)
		fmt.Fprintf(c.b, "  store i64 %%%s, ptr %s\n", t, slot)
	default:
		return fmt.Errorf("arm: unsupported FP result slot type %q", meta.Type)
	}
	c.markFPResultWritten(off)
	return nil
}

func (c *armCtx) loadFPResult(slot FrameSlot) (string, error) {
	p, ok := c.fpResAllocaIdx[slot.Index]
	if !ok {
		return "", fmt.Errorf("arm: missing FP result alloca for index %d", slot.Index)
	}
	t := c.newTmp()
	fmt.Fprintf(c.b, "  %%%s = load %s, ptr %s\n", t, slot.Type, p)
	return "%" + t, nil
}

func (c *armCtx) loadRetSlotFallback(slot FrameSlot) (string, error) {
	v32, err := c.loadReg(Reg("R0"))
	if err != nil {
		return "", err
	}
	switch slot.Type {
	case I32:
		return v32, nil
	case I16, I8, I1:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = trunc i32 %s to %s\n", t, v32, slot.Type)
		return "%" + t, nil
	case Ptr:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = inttoptr i32 %s to ptr\n", t, v32)
		return "%" + t, nil
	case I64:
		t := c.newTmp()
		fmt.Fprintf(c.b, "  %%%s = zext i32 %s to i64\n", t, v32)
		return "%" + t, nil
	default:
		return "", fmt.Errorf("arm: unsupported fallback return type %q", slot.Type)
	}
}
