package fixedlength

import (
	"github.com/jf-tech/go-corelib/maths"

	"github.com/jf-tech/omniparser/extensions/omniv21/fileformat/flatfile"
	"github.com/jf-tech/omniparser/idr"
)

const (
	recDeclTypeRec   = "record"
	recDeclTypeGroup = "record_group"
)

const (
	fqdnDelim   = "/"
	rootRecName = "#root"
)

// FieldDecl describes a field inside an envelope.
type FieldDecl struct {
	Name     string  `json:"name,omitempty"`
	StartPos int     `json:"start_pos"` // 1-based. and rune-based.
	Length   int     `json:"length"`    // rune-based length.
	Default  *string `json:"default,omitempty"`
}

// RecDecl describes an fixed length record declaration/settings.
type RecDecl struct {
	Name     string      `json:"name,omitempty"`
	Type     *string     `json:"type,omitempty"`
	IsTarget bool        `json:"is_target,omitempty"`
	Min      *int        `json:"min,omitempty"`
	Max      *int        `json:"max,omitempty"`
	Fields   []FieldDecl `json:"fields,omitempty"`
	Children []*RecDecl  `json:"child_records,omitempty"`

	fqdn       string // the full hierarchical name to record decl.
	childDecls []flatfile.RecDecl
}

func (d *RecDecl) UniqueName() string {
	return d.fqdn
}

func (d *RecDecl) Target() bool {
	return d.IsTarget
}

func (d *RecDecl) Group() bool {
	return d.Type != nil && *d.Type == recDeclTypeGroup
}

func (d *RecDecl) MinOccurs() int {
	switch d.Min {
	case nil:
		return 1
	default:
		return *d.Min
	}
}

func (d *RecDecl) MaxOccurs() int {
	switch {
	case d.Max == nil:
		return 1
	case *d.Max < 0:
		return maths.MaxIntValue
	default:
		return *d.Max
	}
}

func (d *RecDecl) ChildDecls() []flatfile.RecDecl {
	return d.childDecls
}

func (d *RecDecl) Match(rec interface{}) bool {
	switch d.Group() {
	case true:
		return len(d.ChildDecls()) > 0 && d.ChildDecls()[0].Match(rec)
	default:
		return d.Name == rec.(*raw).name
	}
}

func (d *RecDecl) ToNode(rec interface{}) (*idr.Node, error) {
	raw := rec.(*raw)
	n := idr.CreateNode(idr.ElementNode, d.Name)
	for _, fd := range d.Fields {
		field := idr.CreateNode(idr.ElementNode, fd.Name)
		idr.AddChild(n, field)
		value := idr.CreateNode(
			idr.TextNode, string(raw.lines[0][fd.StartPos-1:fd.StartPos-1+fd.Length]))
		idr.AddChild(field, value)
	}
	return n, nil
}

func toFlatFileRecDecls(ds []*RecDecl) []flatfile.RecDecl {
	if len(ds) == 0 {
		return nil
	}
	ret := make([]flatfile.RecDecl, len(ds))
	for i, d := range ds {
		ret[i] = d
	}
	return ret
}

// FileDecl describes fixed-length specific schema settings for omniparser reader.
type FileDecl struct {
	// TODO more to cater single line, by_rows, and by_headerfooter, etc.
	RecDecls []*RecDecl `json:"record_declarations,omitempty"`
}
