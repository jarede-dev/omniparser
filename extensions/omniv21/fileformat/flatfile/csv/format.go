package csv

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/antchfx/xpath"
	"github.com/jf-tech/go-corelib/caches"
	"github.com/jf-tech/go-corelib/strs"

	"github.com/jf-tech/omniparser/errs"
	"github.com/jf-tech/omniparser/extensions/omniv21/fileformat"
	"github.com/jf-tech/omniparser/extensions/omniv21/transform"
)

const (
	fileFormatCSV = "csv2"
)

type csvFileFormat struct {
	schemaName string
}

// NewCSVFileFormat creates a FileFormat for CSV.
func NewCSVFileFormat(schemaName string) fileformat.FileFormat {
	return &csvFileFormat{schemaName: schemaName}
}

func (f *csvFileFormat) CreateFormatReader(
	name string, r io.Reader, runtime interface{}) (fileformat.FormatReader, error) {
	csv := runtime.(*csvFormatRuntime)
	targetXPathExpr, err := func() (*xpath.Expr, error) {
		if csv.XPath == "" || csv.XPath == "." {
			return nil, nil
		}
		return caches.GetXPathExpr(csv.XPath)
	}()
	if err != nil {
		return nil, f.FmtErr("xpath '%s' on 'FINAL_OUTPUT' is invalid: %s", csv.XPath, err.Error())
	}
	return NewReader(name, r, csv.Decl, targetXPathExpr), nil
}

func (f *csvFileFormat) FmtErr(format string, args ...interface{}) error {
	return fmt.Errorf("schema '%s': %s", f.schemaName, fmt.Sprintf(format, args...))
}

type csvFormatRuntime struct {
	Decl  *FileDecl `json:"file_declaration"`
	XPath string
}

func (f *csvFileFormat) ValidateSchema(
	format string, schemaContent []byte, finalOutputDecl *transform.Decl) (interface{}, error) {
	if format != fileFormatCSV {
		return nil, errs.ErrSchemaNotSupported
	}
	/* TODO
	err := validation.SchemaValidate(f.schemaName, schemaContent, v21validation.JSONSchemaCSVFileDeclaration)
	if err != nil {
		// err is already context formatted.
		return nil, err
	}
	*/
	var runtime csvFormatRuntime
	_ = json.Unmarshal(schemaContent, &runtime) // JSON schema validation earlier guarantees Unmarshal success.
	err := f.validateFileDecl(runtime.Decl)
	if err != nil {
		// err is already context formatted.
		return nil, err
	}
	if finalOutputDecl == nil {
		return nil, f.FmtErr("'FINAL_OUTPUT' is missing")
	}
	runtime.XPath = strings.TrimSpace(strs.StrPtrOrElse(finalOutputDecl.XPath, ""))
	if runtime.XPath != "" {
		_, err := caches.GetXPathExpr(runtime.XPath)
		if err != nil {
			return nil, f.FmtErr("'FINAL_OUTPUT.xpath' (value: '%s') is invalid, err: %s",
				runtime.XPath, err.Error())
		}
	}
	return &runtime, nil
}

func (f *csvFileFormat) validateFileDecl(decl *FileDecl) error {
	err := (&validateCtx{}).validateFileDecl(decl)
	if err != nil {
		return f.FmtErr(err.Error())
	}
	return err
}
