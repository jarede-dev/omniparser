package fixedlength

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/antchfx/xpath"
	"github.com/jf-tech/go-corelib/ios"

	"github.com/jf-tech/omniparser/extensions/omniv21/fileformat/flatfile"
	"github.com/jf-tech/omniparser/idr"
)

// ErrInvalidFixedLength indicates the fixed-length-adv content is corrupted.
// This is a fatal, non-continuable error.
type ErrInvalidFixedLength string

func (e ErrInvalidFixedLength) Error() string { return string(e) }

// IsErrInvalidFixedLength checks if the `err` is of ErrInvalidFixedLength type.
func IsErrInvalidFixedLength(err error) bool {
	switch err.(type) {
	case ErrInvalidFixedLength:
		return true
	default:
		return false
	}
}

type raw struct {
	valid bool
	name  string
	lines [][]byte
}

func (r *raw) reset() {
	r.valid = false
	r.lines = r.lines[:0] // [:0] is safe even on nil.
}

type reader struct {
	inputName string
	r         *bufio.Reader
	hr        *flatfile.HierarchyReader
	lineNum   int
	raw       *raw
}

func (r *reader) Cur() (interface{}, error) {
	if r.raw.valid {
		return r.raw, nil
	}
	// TODO for now just do the simple single line thingy.
read:
	line, err := ios.ByteReadLine(r.r)
	switch {
	case err == io.EOF:
		return nil, io.EOF
	case err != nil:
		return nil, ErrInvalidFixedLength(r.fmtErrStr(err.Error()))
	}
	r.lineNum++
	if len(line) <= 0 {
		// TODO properly honor ignore_crlf flag
		goto read
	}
	r.raw.valid = true
	r.raw.name = string(line[:3]) // TODO
	r.raw.lines = append(r.raw.lines, line)
	return r.raw, nil
}

func (r *reader) MarkCurDone() {
	r.raw.reset()
}

func (r *reader) Read() (*idr.Node, error) {
	n, err := r.hr.Read()
	switch err {
	case nil:
		return n, nil
	case io.EOF:
		return nil, io.EOF
	default:
		return nil, ErrInvalidFixedLength(r.fmtErrStr(err.Error()))
	}
}

func (r *reader) Release(n *idr.Node) {
	r.hr.Release(n)
}

func (r *reader) IsContinuableError(err error) bool {
	return !IsErrInvalidFixedLength(err) && err != io.EOF
}

func (r *reader) FmtErr(format string, args ...interface{}) error {
	return errors.New(r.fmtErrStr(format, args...))
}

func (r *reader) fmtErrStr(format string, args ...interface{}) string {
	return fmt.Sprintf("input '%s' line %d: %s",
		r.inputName, r.lineNum, fmt.Sprintf(format, args...))
}

// NewReader creates an FormatReader for 'fixed-length-adv' file format.
func NewReader(
	inputName string, r io.Reader, decl *FileDecl, targetXPathExpr *xpath.Expr) (*reader, error) {
	reader := &reader{
		inputName: inputName,
		r:         bufio.NewReader(r),
		raw:       &raw{},
	}
	reader.hr = flatfile.NewHierarchyReader(
		toFlatFileRecDecls(decl.RecDecls), reader, targetXPathExpr)
	return reader, nil
}
