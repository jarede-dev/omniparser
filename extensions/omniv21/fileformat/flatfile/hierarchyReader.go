package flatfile

import (
	"fmt"
	"io"

	"github.com/antchfx/xpath"

	"github.com/jf-tech/omniparser/idr"
)

type ErrFewerThanMinOccurs struct {
	RecDecl       RecDecl
	ActualOcccurs int
}

// Error satisfies the error interface with a dummy text. User of this error must
// directly access the payload and reconstruct their own context aware error.
func (e ErrFewerThanMinOccurs) Error() string { panic("shouldn't be called") }

func IsErrFewerThanMinOccurs(err error) bool {
	switch err.(type) {
	case ErrFewerThanMinOccurs:
		return true
	default:
		return false
	}
}

type ErrUnexpectedData struct{}

func IsErrUnexpectedData(err error) bool {
	switch err.(type) {
	case ErrUnexpectedData:
		return true
	default:
		return false
	}
}

// Error satisfies the error interface with a dummy text. User of this error must
// directly access the payload and reconstruct their own context aware error.
func (e ErrUnexpectedData) Error() string { panic("shouldn't be called") }

type HierarchyReader struct {
	r               RecReader
	stack           []stackEntry
	target          *idr.Node
	targetXPathExpr *xpath.Expr
}

func NewHierarchyReader(
	children []RecDecl, recReader RecReader, targetXPathExpr *xpath.Expr) *HierarchyReader {
	r := &HierarchyReader{
		r:               recReader,
		stack:           make([]stackEntry, 0, initialStackDepth),
		targetXPathExpr: targetXPathExpr,
	}
	rootDecl := rootDecl{children: children}
	r.growStack(stackEntry{
		recDecl: rootDecl,
		recNode: idr.CreateNode(idr.DocumentNode, rootDecl.DeclName()),
	})
	if len(rootDecl.children) > 0 {
		r.growStack(stackEntry{recDecl: rootDecl.children[0]})
	}
	return r
}

// Read .....
func (r *HierarchyReader) Read() (*idr.Node, error) {
	if r.target != nil {
		// This is just in case Release() isn't called by ingester.
		idr.RemoveAndReleaseTree(r.target)
		r.target = nil
	}
	for {
		if r.target != nil {
			return r.target, nil
		}
		more, err := r.r.MoreUnprocessedData()
		if err != nil {
			// r.r.MoreUnprocessedData() has encounter some real IO failures.
			return nil, err
		}
		if !more {
			// When the input is done, we still need to verified all the
			// remaining decls' min occurs are satisfied. We can do so by
			// simply keeping on moving to the next rec: we call recNext()
			// once at a time - in case after the recNext() call, the reader
			// yields another target node. We can safely do this (1 recNext()
			// call at a time after we encounter EOF) is because
			// r.r.MoreUnprocessedData() should/will repeatedly return io.EOF,
			// once it returns io.EOF.
			if len(r.stack) <= 1 {
				// If we don't have any more data, and our decl stack has been
				// completed, then we're all done!!
				return nil, io.EOF
			}
			err = r.recNext()
			if err != nil {
				return nil, err
			}
			continue
		}
		// Now at this point, we know we have more unprocessed data.
		if len(r.stack) <= 1 {
			// This means we currently have some unprocessed data but all the rec
			// decls' processing is done.
			return nil, ErrUnexpectedData{}
		}
		curRecEntry := r.stackTop()
		node, err := r.readRec(curRecEntry.recDecl)
		// Note given we have unprocessed data, r.r.ReadRec should never return
		// io.EOF. So any error encountered, we directly bail out.
		if err != nil {
			return nil, err
		}
		// if no err returned from r.r.ReadRec(), but node returned is nil, that means
		// the current data isn't a match for the curRecEntry.recDecl. So the
		// curRecEntry.recDecl instance is considered done.
		if node == nil {
			err = r.recNext() // move onto the decl's next instance.
			if err != nil {
				return nil, err
			}
			continue
		}
		curRecEntry.recNode = node
		// the new idr node is a new instance of the current RecDecl thus when we add it to
		// the IDR tree, we need to add it as a child of the current RecDecl's parent, thus
		// adding it to stackTop(1), not (0).
		idr.AddChild(r.stackTop(1).recNode, curRecEntry.recNode)
		if len(curRecEntry.recDecl.ChildDecls()) > 0 {
			r.growStack(stackEntry{recDecl: curRecEntry.recDecl.ChildDecls()[0]})
			continue
		}
		r.recDone()
	}
}

func (r *HierarchyReader) Release(n *idr.Node) {
	if n == nil {
		return
	}
	if r.target == n {
		r.target = nil
	}
	idr.RemoveAndReleaseTree(n)
}

func (r *HierarchyReader) readRec(recDecl RecDecl) (*idr.Node, error) {
	// If the decl is a solid non-group decl, then we will ask RecReader to match and create IDR.
	// If the decl is a group, then we'll only ask RecReader to match but not creating IDR - instead
	// we'll create an IDR node for the group decl here.
	nonGroupDecl := recDecl
	for nonGroupDecl.Group() && len(nonGroupDecl.ChildDecls()) > 0 {
		nonGroupDecl = nonGroupDecl.ChildDecls()[0]
	}
	if nonGroupDecl.Group() {
		// We must have a non-group solid record to perform the match; if not, it's a no
		// match, thus returning nil for IDR and nil for error.
		return nil, nil
	}
	// Now we have a solid record to perform match, let's call RecReader to do that.
	// In case the recDecl is the solid non-group record itself, we can ask RecReader
	// to perform the match and IDR node creation at the same time.
	matched, node, err := r.r.ReadAndMatch(nonGroupDecl, !recDecl.Group())
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}
	if recDecl.Group() {
		return idr.CreateNode(idr.ElementNode, recDecl.DeclName()), nil
	}
	return node, nil
}

type stackEntry struct {
	recDecl  RecDecl   // the current stack entry's record decl
	recNode  *idr.Node // the current stack entry record's IDR node
	curChild int       // which child record is the current record is processing.
	occurred int       // how many times the current record is fully processed.
}

const (
	initialStackDepth = 10
)

// stackTop returns the pointer to the 'frame'-th stack entry from the top.
// 'frame' is optional, if not specified, default 0 (aka the very top of
// the stack) is assumed. Note caller NEVER owns the memory of the returned
// entry, thus caller can use the pointer and its data values inside locally
// but should never cache/save it somewhere for later usage.
func (r *HierarchyReader) stackTop(frame ...int) *stackEntry {
	nth := 0
	if len(frame) == 1 {
		nth = frame[0]
	}
	if nth < 0 || nth >= len(r.stack) {
		panic(fmt.Sprintf("frame requested: %d, but stack length: %d", nth, len(r.stack)))
	}
	return &r.stack[len(r.stack)-nth-1]
}

// shrinkStack removes the top frame of the stack and returns the pointer to the NEW TOP
// FRAME to caller. Note caller NEVER owns the memory of the returned entry, thus caller can
// use the pointer and its data values inside locally but should never cache/save it somewhere
// for later usage.
func (r *HierarchyReader) shrinkStack() *stackEntry {
	if len(r.stack) < 1 {
		panic("stack length is empty")
	}
	r.stack = r.stack[:len(r.stack)-1]
	if len(r.stack) < 1 {
		return nil
	}
	return &r.stack[len(r.stack)-1]
}

// growStack adds a new stack entry to the top of the stack.
func (r *HierarchyReader) growStack(e stackEntry) {
	r.stack = append(r.stack, e)
}

// recDone wraps up the processing of an instance of current record (which includes the processing of
// the instances of its child records). recDone marks streaming target if necessary. If the number of
// instance occurrences is over the current record's max limit, recDone calls recNext to move to the
// next record in sequence; If the number of instances is still within max limit, recDone does no more
// action so the current record will remain on top of the stack and potentially process more instances
// of this record. Note: recDone is potentially recursive: recDone -> recNext -> recDone -> ...
func (r *HierarchyReader) recDone() {
	cur := r.stackTop()
	cur.curChild = 0
	cur.occurred++
	if cur.recDecl.Target() {
		if r.target != nil {
			panic("r.target != nil")
		}
		if cur.recNode == nil {
			panic("cur.recNode == nil")
		}
		if r.targetXPathExpr == nil || idr.MatchAny(cur.recNode, r.targetXPathExpr) {
			r.target = cur.recNode
		} else {
			idr.RemoveAndReleaseTree(cur.recNode)
			cur.recNode = nil
		}
	}
	if cur.occurred < cur.recDecl.MaxOccurs() {
		return
	}
	// we're here because `cur.occurred >= cur.recDecl.MaxOccurs()`
	// and the only path recNext() can fail is to have
	// `cur.occurred < cur.recDecl.MinOccurs()`, which means
	// the calling to recNext() from recDone() will never fail,
	// if our validation makes sure min<=max.
	_ = r.recNext()
}

// recNext is called when the top-of-stack (aka current) record is done its full processing and needs to move
// to the next record. If the current record has a subsequent sibling, that sibling will be the next record;
// If not, it indicates the current record's parent record is fully done its processing, thus parent's recDone
// is called. Note: recNext is potentially recursive: recNext -> recDone -> recNext -> ...
func (r *HierarchyReader) recNext() error {
	cur := r.stackTop()
	if cur.occurred < cur.recDecl.MinOccurs() {
		return ErrFewerThanMinOccurs{RecDecl: cur.recDecl, ActualOcccurs: cur.occurred}
	}
	if len(r.stack) <= 1 {
		return nil
	}
	cur = r.shrinkStack()
	if cur.curChild < len(cur.recDecl.ChildDecls())-1 {
		cur.curChild++
		r.growStack(stackEntry{recDecl: cur.recDecl.ChildDecls()[cur.curChild]})
		return nil
	}
	r.recDone()
	return nil
}
