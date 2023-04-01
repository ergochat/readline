package readline

type undoEntry struct {
	pos int
	buf []rune
}

type opUndo struct {
	op    *operation
	stack []undoEntry
}

func newOpUndo(op *operation) *opUndo {
	o := &opUndo{op: op}
	o.init()
	return o
}

func (o *opUndo) add(pos int, buf []rune) {
	o.stack = append(o.stack, undoEntry{pos: pos, buf: buf})
}

func (o *opUndo) undo() {
	if len(o.stack) == 0 {
		return
	}

	e := o.stack[len(o.stack)-1]
	o.stack = o.stack[0 : len(o.stack)-1]

	o.op.buf.buf = e.buf
	o.op.buf.idx = e.pos
	o.op.buf.Refresh(nil)
}

func (o *opUndo) init() {
	o.stack = []undoEntry{
		{
			pos: o.op.buf.idx,
			buf: append([]rune{}, o.op.buf.buf...),
		},
	}
}
