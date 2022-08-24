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

// ErrInvalidFixedLength indicates the fixed-length content is corrupted or IO failure.
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

type line struct {
	lineNum int // 1-based
	b       []byte
}

type reader struct {
	inputName string
	r         *bufio.Reader
	hr        *flatfile.HierarchyReader
	linesRead int    // total number of lines read in, including those already in linesBuf.
	linesBuf  []line // linesBuf contains all the unprocessed lines
}

// NewReader creates an FormatReader for fixed-length file format.
func NewReader(
	inputName string, r io.Reader, decl *FileDecl, targetXPathExpr *xpath.Expr) *reader {
	reader := &reader{
		inputName: inputName,
		r:         bufio.NewReader(r),
	}
	reader.hr = flatfile.NewHierarchyReader(
		toFlatFileRecDecls(decl.Envelopes), reader, targetXPathExpr)
	return reader
}

func (r *reader) MoreUnprocessedData() (bool, error) {
	if len(r.linesBuf) > 0 {
		return true, nil
	}
	if err := r.readLine(); err != nil && err != io.EOF {
		return false, err
	}
	return len(r.linesBuf) > 0, nil
}

func (r *reader) ReadAndMatch(
	decl flatfile.RecDecl, createIDR bool) (matched bool, node *idr.Node, err error) {
	envelopeDecl := decl.(*EnvelopeDecl)
	if envelopeDecl.rowsBased() {
		return r.readAndMatchRowsBasedEnvelope(envelopeDecl, createIDR)
	}
	return r.readAndMatchHeaderFooterBasedEnvelope(envelopeDecl, createIDR)
}

func (r *reader) readAndMatchRowsBasedEnvelope(
	decl *EnvelopeDecl, createNode bool) (bool, *idr.Node, error) {
	for len(r.linesBuf) < decl.rows() {
		if err := r.readLine(); err != nil {
			if err != io.EOF || len(r.linesBuf) == 0 {
				// So either not an io.EOF, we need to return the critical error directly;
				// or it's EOF and our line buf empty, i.e. all has been procssed, so we
				// can directly return io.EOF to indicate end.
				return false, nil, err
			}
			// we're here if the err is io.EOF and line buf isn't empty, so we return
			// we're no match, and no error, hoping the non-empty line buf will be matched
			// by subsequent decl calls.
			return false, nil, nil
		}
	}
	if createNode {
		n := r.linesToNode(decl, decl.rows())
		r.popFrontLinesBuf(decl.rows())
		return true, n, nil
	}
	return true, nil, nil
}

func (r *reader) readAndMatchHeaderFooterBasedEnvelope(
	decl *EnvelopeDecl, createNode bool) (bool, *idr.Node, error) {
	if len(r.linesBuf) <= 0 {
		if err := r.readLine(); err != nil {
			// io.EOF or not, since r.linesBuf is empty, we can directly return err.
			return false, nil, err
		}
	}
	if !decl.matchHeader(r.linesBuf[0].b) {
		return false, nil, nil
	}
	i := 0
	for {
		if decl.matchFooter(r.linesBuf[i].b) {
			if createNode {
				n := r.linesToNode(decl, i+1)
				r.popFrontLinesBuf(i + 1)
				return true, n, nil
			}
			return true, nil, nil
		}
		// if by the end of r.linesBuf we still haven't matched footer, we need to
		// read more line in for footer match.
		if i >= len(r.linesBuf)-1 {
			if err := r.readLine(); err != nil {
				if err != io.EOF { // io reading error, directly return err.
					return false, nil, err
				}
				// io.EOF encountered and since r.linesBuf isn't empty,
				// we need to return false for matching, but nil for error (we only return io.EOF
				// when r.linesBuf is empty.
				return false, nil, nil
			}
		}
		i++
	}
}

func (r *reader) readLine() error {
	for {
		// note1: ios.ByteReadLine returns a ln with trailing '\n' (and/or '\r') dropped.
		// note2: ios.ByteReadLine won't return io.EOF if ln returned isn't empty.
		b, err := ios.ByteReadLine(r.r)
		switch {
		case err == io.EOF:
			return io.EOF
		case err != nil:
			return ErrInvalidFixedLength(r.fmtErrStr(r.linesRead+1, err.Error()))
		}
		r.linesRead++
		if len(b) > 0 {
			r.linesBuf = append(r.linesBuf, line{lineNum: r.linesRead, b: b})
			return nil
		}
	}
}

func (r *reader) linesToNode(decl *EnvelopeDecl, n int) *idr.Node {
	node := idr.CreateNode(idr.ElementNode, decl.Name)
	columnsDone := make([]bool, len(decl.Columns))
	for col, _ := range decl.Columns {
		if columnsDone[col] {
			continue
		}
		colDecl := decl.Columns[col]
		for i := 0; i < n; i++ {
			if !colDecl.lineMatch(r.linesBuf[i].b) {
				continue
			}
			colNode := idr.CreateNode(idr.ElementNode, colDecl.Name)
			idr.AddChild(node, colNode)
			colVal := idr.CreateNode(idr.TextNode, colDecl.lineToColumnValue(r.linesBuf[i].b))
			idr.AddChild(colNode, colVal)
			columnsDone[col] = true
		}
	}
	return node
}

func (r *reader) popFrontLinesBuf(n int) {
	if n > len(r.linesBuf) {
		panic("less r.linesBuf than requested pop front")
	}
	newLen := len(r.linesBuf) - n
	for i := 0; i < newLen; i++ {
		r.linesBuf[i] = r.linesBuf[i+n]
	}
	r.linesBuf = r.linesBuf[:newLen]
}

func (r *reader) Read() (*idr.Node, error) {
	n, err := r.hr.Read()
	switch {
	case err == nil:
		return n, nil
	case err == io.EOF:
		return nil, io.EOF
	case flatfile.IsErrFewerThanMinOccurs(err):
		e := err.(flatfile.ErrFewerThanMinOccurs)
		envelopeDecl := e.RecDecl.(*EnvelopeDecl)
		return nil, ErrInvalidFixedLength(r.fmtErrStr(r.unprocessedLineNum(),
			"envelope/envelope_group '%s' needs min occur %d, but only got %d",
			envelopeDecl.fqdn, envelopeDecl.MinOccurs(), e.ActualOcccurs))
	case flatfile.IsErrUnexpectedData(err):
		return nil, ErrInvalidFixedLength(r.fmtErrStr(r.unprocessedLineNum(), "unexpected data"))
	default:
		return nil, ErrInvalidFixedLength(r.fmtErrStr(r.unprocessedLineNum(), err.Error()))
	}
}

func (r *reader) unprocessedLineNum() int {
	if len(r.linesBuf) > 0 {
		return r.linesBuf[0].lineNum
	}
	return r.linesRead + 1
}

func (r *reader) Release(n *idr.Node) {
	r.hr.Release(n)
}

func (r *reader) IsContinuableError(err error) bool {
	return !IsErrInvalidFixedLength(err) && err != io.EOF
}

func (r *reader) FmtErr(format string, args ...interface{}) error {
	return errors.New(r.fmtErrStr(r.unprocessedLineNum(), format, args...))
}

func (r *reader) fmtErrStr(line int, format string, args ...interface{}) string {
	return fmt.Sprintf("input '%s' line %d: %s",
		r.inputName, line, fmt.Sprintf(format, args...))
}
