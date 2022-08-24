package csv

import (
	"errors"
	"fmt"
	"io"

	"github.com/antchfx/xpath"
	"github.com/jf-tech/go-corelib/ios"

	"github.com/jf-tech/omniparser/extensions/omniv21/fileformat/flatfile"
	"github.com/jf-tech/omniparser/idr"
)

// ErrInvalidCSV indicates the csv content is corrupted or an IO failure occurred.
// This is a fatal, non-continuable error.
type ErrInvalidCSV string

func (e ErrInvalidCSV) Error() string { return string(e) }

// IsErrInvalidCSV checks if the `err` is of ErrInvalidCSV type.
func IsErrInvalidCSV(err error) bool {
	switch err.(type) {
	case ErrInvalidCSV:
		return true
	default:
		return false
	}
}

type reader struct {
	inputName string
	r         *ios.LineNumReportingCsvReader
	hr        *flatfile.HierarchyReader
	rawRecord []string
}

// NewReader creates an FormatReader for csv file format.
func NewReader(
	inputName string, r io.Reader, decl *FileDecl, targetXPathExpr *xpath.Expr) *reader {

	if decl.ReplaceDoubleQuotes {
		r = ios.NewBytesReplacingReader(r, []byte(`"`), []byte(`'`))
	}
	csvReader := ios.NewLineNumReportingCsvReader(r)
	csvReader.Comma = []rune(decl.Delimiter)[0]
	csvReader.FieldsPerRecord = -1
	csvReader.ReuseRecord = true
	reader := &reader{
		inputName: inputName,
		r:         csvReader,
	}
	reader.hr = flatfile.NewHierarchyReader(
		toFlatFileRecDecls(decl.Records), reader, targetXPathExpr)
	return reader
}

func (r *reader) MoreUnprocessedData() (bool, error) {
	if err := r.readCSV(); err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *reader) ReadAndMatch(decl flatfile.RecDecl, createIDR bool) (bool, *idr.Node, error) {
	recordDecl := decl.(*RecordDecl)
	if err := r.readCSV(); err != nil {
		return false, nil, err
	}
	for i, col := range recordDecl.Columns {
		if i < len(r.rawRecord) && !col.match(r.rawRecord[i]) {
			return false, nil, nil
		}
		if i >= len(r.rawRecord) && col.Match != nil {
			return false, nil, nil
		}
	}
	var n *idr.Node
	if createIDR {
		n = idr.CreateNode(idr.ElementNode, recordDecl.Name)
		for i, col := range recordDecl.Columns {
			if i >= len(r.rawRecord) {
				break
			}
			e := idr.CreateNode(idr.ElementNode, col.name())
			idr.AddChild(n, e)
			v := idr.CreateNode(idr.TextNode, r.rawRecord[i])
			idr.AddChild(e, v)
		}
		r.rawRecord = nil
	}
	return true, n, nil
}

func (r *reader) readCSV() (err error) {
	if len(r.rawRecord) > 0 {
		return nil
	}
	r.rawRecord, err = r.r.Read()
	if err != nil && err != io.EOF {
		return ErrInvalidCSV(err.Error())
	}
	return err
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
		recordDecl := e.RecDecl.(*RecordDecl)
		return nil, ErrInvalidCSV(r.fmtErrStr(
			"record/record_group '%s' needs min occur %d, but only got %d",
			recordDecl.fqdn, recordDecl.MinOccurs(), e.ActualOcccurs))
	case flatfile.IsErrUnexpectedData(err):
		return nil, ErrInvalidCSV(r.fmtErrStr("unexpected data"))
	default:
		return nil, ErrInvalidCSV(r.fmtErrStr(err.Error()))
	}
}

func (r *reader) Release(n *idr.Node) {
	r.hr.Release(n)
}

func (r *reader) IsContinuableError(err error) bool {
	return !IsErrInvalidCSV(err) && err != io.EOF
}

func (r *reader) FmtErr(format string, args ...interface{}) error {
	return errors.New(r.fmtErrStr(format, args...))
}

func (r *reader) fmtErrStr(format string, args ...interface{}) string {
	return fmt.Sprintf(
		"input '%s' line %d: %s", r.inputName, r.r.LineNum(), fmt.Sprintf(format, args...))
}
