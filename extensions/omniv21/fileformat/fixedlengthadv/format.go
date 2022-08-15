package fixedlengthadv

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jf-tech/go-corelib/caches"
	"github.com/jf-tech/go-corelib/strs"

	"github.com/jf-tech/omniparser/errs"
	"github.com/jf-tech/omniparser/extensions/omniv21/fileformat"
	"github.com/jf-tech/omniparser/extensions/omniv21/transform"
)

const (
	fileFormatFixedLengthAdv = "fixed-length-adv"
)

type fixedLengthAdvFormat struct {
	schemaName string
}

// NewFixedLengthAdvFormat creates a FileFormat for 'fixed-length-adv'.
func NewFixedLengthAdvFormat(schemaName string) fileformat.FileFormat {
	return &fixedLengthAdvFormat{schemaName: schemaName}
}

type fixedLengthAdvFormatRuntime struct {
	Decl  *FileDecl `json:"file_declaration"`
	XPath string
}

func (f *fixedLengthAdvFormat) ValidateSchema(
	format string, schemaContent []byte, finalOutputDecl *transform.Decl) (interface{}, error) {
	if format != fileFormatFixedLengthAdv {
		return nil, errs.ErrSchemaNotSupported
	}
	/* TODO
	err := validation.SchemaValidate(f.schemaName, schemaContent, v21validation.JSONSchemaFixedLengthAdvFileDeclaration)
	if err != nil {
		// err is already context formatted.
		return nil, err
	}*/
	var runtime fixedLengthAdvFormatRuntime
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

func (f *fixedLengthAdvFormat) validateFileDecl(decl *FileDecl) error {
	err := (&validateCtx{}).validateFileDecl(decl)
	if err != nil {
		return f.FmtErr(err.Error())
	}
	return err
}

func (f *fixedLengthAdvFormat) CreateFormatReader(
	name string, r io.Reader, runtime interface{}) (fileformat.FormatReader, error) {
	rt := runtime.(*fixedLengthAdvFormatRuntime)
	return NewReader(name, r, rt.Decl, rt.XPath)
}

func (f *fixedLengthAdvFormat) FmtErr(format string, args ...interface{}) error {
	return fmt.Errorf("schema '%s': %s", f.schemaName, fmt.Sprintf(format, args...))
}
