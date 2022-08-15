package fixedlengthadv

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/antchfx/xpath"
	"github.com/jf-tech/go-corelib/caches"
	"github.com/jf-tech/go-corelib/ios"
	"github.com/jf-tech/go-corelib/strs"

	"github.com/jf-tech/omniparser/idr"
)

type stackEntry struct {
	recDecl  *RecDecl  // the current stack entry's record decl
	recNode  *idr.Node // the current stack entry record's IDR node
	curChild int       // which child record is the current record is processing.
	occurred int       // how many times the current record is fully processed.
}

const (
	defaultStackDepth = 10
)

func newStack() []stackEntry {
	return make([]stackEntry, 0, defaultStackDepth)
}

type reader struct {
	inputName         string
	r                 *bufio.Reader
	stack             []stackEntry
	target            *idr.Node
	targetXPath       *xpath.Expr
	unprocessedRawRec RawRec
}

func inRange(i, lowerBoundInclusive, upperBoundInclusive int) bool {
	return i >= lowerBoundInclusive && i <= upperBoundInclusive
}

// stackTop returns the pointer to the 'frame'-th stack entry from the top.
// 'frame' is optional, if not specified, default 0 (aka the very top of
// the stack) is assumed. Note caller NEVER owns the memory of the returned
// entry, thus caller can use the pointer and its data values inside locally
// but should never cache/save it somewhere for later usage.
func (r *reader) stackTop(frame ...int) *stackEntry {
	nth := 0
	if len(frame) == 1 {
		nth = frame[0]
	}
	if !inRange(nth, 0, len(r.stack)-1) {
		panic(fmt.Sprintf("frame requested: %d, but stack length: %d", nth, len(r.stack)))
	}
	return &r.stack[len(r.stack)-nth-1]
}

// shrinkStack removes the top frame of the stack and returns the pointer to the NEW TOP
// FRAME to caller. Note caller NEVER owns the memory of the returned entry, thus caller can
// use the pointer and its data values inside locally but should never cache/save it somewhere
// for later usage.
func (r *reader) shrinkStack() *stackEntry {
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
func (r *reader) growStack(e stackEntry) {
	r.stack = append(r.stack, e)
}

func (r *reader) resetRawRec() {
	resetRawRec(&r.unprocessedRawRec)
}

func (r *reader) getUnprocessedRawRec() (RawRec, error) {
	if r.unprocessedRawRec.valid {
		return r.unprocessedRawRec, nil
	}
read:
	line, err := ios.ByteReadLine(r.r)
	switch {
	case err == io.EOF:
		return RawRec{}, io.EOF
	case err != nil:
		return RawRec{}, ErrInvalidFixedLengthAdv(r.fmtErrStr(err.Error()))
	}
	if len(line) <= 0 {
		goto read
	}
	r.unprocessedRawRec = RawRec{
		valid: true,
		Name:  string(line[:3]), // TODO
		Raw:   line,
	}
	return r.unprocessedRawRec, nil
}

func (r *reader) rawRecToNode(recDecl *RecDecl) (*idr.Node, error) {
	if !r.unprocessedRawRec.valid {
		panic("unprocessedRawRec is not valid")
	}
	raw := r.unprocessedRawRec.Raw
	n := idr.CreateNode(idr.ElementNode, recDecl.Name)
	for _, fieldDecl := range recDecl.Fields {
		fieldN := idr.CreateNode(idr.ElementNode, fieldDecl.Name)
		idr.AddChild(n, fieldN)
		start := fieldDecl.StartPos - 1
		end := start + fieldDecl.Length
		data := string(raw[start:end]) // TODO: this is wrong/hack. need to use rune.
		fieldV := idr.CreateNode(idr.TextNode, data)
		idr.AddChild(fieldN, fieldV)
	}
	return n, nil
}

// recDone wraps up the processing of an instance of current record (which includes the processing of
// the instances of its child records). recDone marks streaming target if necessary. If the number of
// instance occurrences is over the current record's max limit, recDone calls recNext to move to the
// next record in sequence; If the number of instances is still within max limit, recDone does no more
// action so the current record will remain on top of the stack and potentially process more instances
// of this record. Note: recDone is potentially recursive: recDone -> recNext -> recDone -> ...
func (r *reader) recDone() {
	cur := r.stackTop()
	cur.curChild = 0
	cur.occurred++
	if cur.recDecl.IsTarget {
		if r.target != nil {
			panic("r.target != nil")
		}
		if cur.recNode == nil {
			panic("cur.recNode == nil")
		}
		if r.targetXPath == nil || idr.MatchAny(cur.recNode, r.targetXPath) {
			r.target = cur.recNode
		} else {
			idr.RemoveAndReleaseTree(cur.recNode)
			cur.recNode = nil
		}
	}
	if cur.occurred < cur.recDecl.maxOccurs() {
		return
	}
	// we're here because `cur.occurred >= cur.recDecl.maxOccurs()`
	// and the only path recNext() can fail is to have
	// `cur.occurred < cur.recDecl.minOccurs()`, which means
	// the calling to recNext() from recDone() will never fail,
	// if our validation makes sure min<=max.
	_ = r.recNext()
}

// recNext is called when the top-of-stack (aka current) record is done its full processing and needs to move
// to the next record. If the current record has a subsequent sibling, that sibling will be the next record;
// If not, it indicates the current record's parent record is fully done its processing, thus parent's recDone
// is called. Note: recNext is potentially recursive: recNext -> recDone -> recNext -> ...
func (r *reader) recNext() error {
	cur := r.stackTop()
	if cur.occurred < cur.recDecl.minOccurs() {
		// TODO work on error msg
		return ErrInvalidFixedLengthAdv(fmt.Sprintf(
			"record '%s' needs min occur %d, but only got %d",
			strs.FirstNonBlank(cur.recDecl.fqdn, cur.recDecl.Name), cur.recDecl.minOccurs(), cur.occurred))
	}
	if len(r.stack) <= 1 {
		return nil
	}
	cur = r.shrinkStack()
	if cur.curChild < len(cur.recDecl.Children)-1 {
		cur.curChild++
		r.growStack(stackEntry{recDecl: cur.recDecl.Children[cur.curChild]})
		return nil
	}
	r.recDone()
	return nil
}

// Read processes input and returns an instance of the streaming target (aka the record marked with is_target=true)
// The basic idea is a forever for-loop, inside which it reads out an unprocessed record data, tries to see
// if the record data matches what's the current record decl we're processing: if matches, great, creates a new
// instance of the current record decl with the data; if not, we call recNext to move the next record decl inline, and
// continue the for-loop so next iteration, the same unprocessed data will be matched against the new record decl.
func (r *reader) Read() (*idr.Node, error) {
	if r.target != nil {
		// This is just in case Release() isn't called by ingester.
		idr.RemoveAndReleaseTree(r.target)
		r.target = nil
	}
	for {
		if r.target != nil {
			return r.target, nil
		}
		rawRec, err := r.getUnprocessedRawRec()
		if err == io.EOF {
			// When the input is done, we still need to verified all the
			// remaining records' min occurs are satisfied. We can do so by
			// simply keeping on moving to the next rec: we call recNext()
			// once at a time - in case after the recNext() call, the reader
			// yields another target node. We can safely do this (1 recNext()
			// call at a time after we encounter EOF) is because getUnprocessedRawRec()
			// will repeatedly return EOF.
			if len(r.stack) <= 1 {
				return nil, io.EOF
			}
			err = r.recNext()
			if err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		cur := r.stackTop()
		if !cur.recDecl.matchRecName(rawRec.Name) {
			if len(r.stack) <= 1 {
				return nil, ErrInvalidFixedLengthAdv(fmt.Sprintf(
					"record '%s' is either not declared in schema or appears in an invalid order",
					rawRec.Name))
			}
			err = r.recNext()
			if err != nil {
				return nil, err
			}
			continue
		}
		if !cur.recDecl.isGroup() {
			cur.recNode, err = r.rawRecToNode(cur.recDecl)
			if err != nil {
				return nil, err
			}
			r.resetRawRec()
		} else {
			cur.recNode = idr.CreateNode(idr.ElementNode, cur.recDecl.Name)
		}
		if len(r.stack) > 1 {
			idr.AddChild(r.stackTop(1).recNode, cur.recNode)
		}
		if len(cur.recDecl.Children) > 0 {
			r.growStack(stackEntry{recDecl: cur.recDecl.Children[0]})
			continue
		}
		r.recDone()
	}
}

func (r *reader) Release(n *idr.Node) {
	if r.target == n {
		r.target = nil
	}
	idr.RemoveAndReleaseTree(n)
}

func (r *reader) IsContinuableError(err error) bool {
	return !IsErrInvalidFixedLengthAdv(err) && err != io.EOF
}

func (r *reader) FmtErr(format string, args ...interface{}) error {
	return errors.New(r.fmtErrStr(format, args...))
}

func (r *reader) fmtErrStr(format string, args ...interface{}) string {
	return fmt.Sprintf("input '%s': %s", r.inputName, fmt.Sprintf(format, args...))
}

// NewReader creates an FormatReader for FixedLengthAdv file format.
func NewReader(inputName string, r io.Reader, decl *FileDecl, targetXPath string) (*reader, error) {
	targetXPathExpr, err := func() (*xpath.Expr, error) {
		if targetXPath == "" || targetXPath == "." {
			return nil, nil
		}
		return caches.GetXPathExpr(targetXPath)
	}()
	if err != nil {
		return nil, fmt.Errorf("invalid target xpath '%s', err: %s", targetXPath, err.Error())
	}
	reader := &reader{
		inputName:   inputName,
		r:           bufio.NewReader(r),
		stack:       newStack(),
		targetXPath: targetXPathExpr,
	}
	reader.growStack(stackEntry{
		recDecl: &RecDecl{
			Name:     rootRecName,
			Type:     strs.StrPtr(recTypeGroup),
			Children: decl.RecDecls,
			fqdn:     rootRecName,
		},
		recNode: idr.CreateNode(idr.DocumentNode, rootRecName),
	})
	if len(decl.RecDecls) > 0 {
		reader.growStack(stackEntry{
			recDecl: decl.RecDecls[0],
		})
	}
	return reader, nil
}
