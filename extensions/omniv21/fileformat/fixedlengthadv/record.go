package fixedlengthadv

import (
	"github.com/jf-tech/go-corelib/maths"
)

// variable/func naming guide:
//
// full name      | short name
// -----------------------------------
// record         | rec
// node           | n
// reader         | r
// current        | cur
// number         | num
// declaration    | decl
// character      | char

const (
	recTypeRec   = "record"
	recTypeGroup = "record_group"
)

const (
	fqdnDelim   = "/"
	rootRecName = "#root"
)

// Field describes a field inside a record.
type Field struct {
	Name     string  `json:"name,omitempty"`
	StartPos int     `json:"start_pos"` // 1-based. and rune-based.
	Length   int     `json:"length"`    // rune-based length.
	Default  *string `json:"default,omitempty"`
}

// RecDecl describes an fixed length record declaration/settings.
type RecDecl struct {
	Name     string     `json:"name,omitempty"`
	Type     *string    `json:"type,omitempty"`
	IsTarget bool       `json:"is_target,omitempty"`
	Min      *int       `json:"min,omitempty"`
	Max      *int       `json:"max,omitempty"`
	Fields   []Field    `json:"fields,omitempty"`
	Children []*RecDecl `json:"child_records,omitempty"`
	fqdn     string     // internal computed field
}

func (d *RecDecl) isGroup() bool {
	return d.Type != nil && *d.Type == recTypeGroup
}

func (d *RecDecl) minOccurs() int {
	switch d.Min {
	case nil:
		return 1
	default:
		return *d.Min
	}
}

func (d *RecDecl) maxOccurs() int {
	switch {
	case d.Max == nil:
		return 1
	case *d.Max < 0:
		return maths.MaxIntValue
	default:
		return *d.Max
	}
}

func (d *RecDecl) matchRecName(recName string) bool {
	switch d.isGroup() {
	case true:
		return len(d.Children) > 0 && d.Children[0].matchRecName(recName)
	default:
		return d.Name == recName
	}
}
