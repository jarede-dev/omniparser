package csv

import (
	"regexp"

	"github.com/jf-tech/go-corelib/maths"
	"github.com/jf-tech/go-corelib/strs"

	"github.com/jf-tech/omniparser/extensions/omniv21/fileformat/flatfile"
)

// ColumnDecl is a CSV column.
type ColumnDecl struct {
	Name  *string `json:"name,omitempty"`
	Match *string `json:"match,omitempty"`

	matchRegexp *regexp.Regexp
}

func (c *ColumnDecl) name() string {
	return strs.StrPtrOrElse(c.Name, "")
}

func (c *ColumnDecl) match(s string) bool {
	if c.matchRegexp == nil {
		return true
	}
	return c.matchRegexp.MatchString(s)
}

const (
	typeRecord = "record"
	typeGroup  = "record_group"
)

type RecordDecl struct {
	Name     string        `json:"name,omitempty"`
	Type     *string       `json:"type,omitempty"`
	IsTarget bool          `json:"is_target,omitempty"`
	Min      *int          `json:"min,omitempty"`
	Max      *int          `json:"max,omitempty"`
	Columns  []*ColumnDecl `json:"columns,omitempty"`
	Children []*RecordDecl `json:"child_records,omitempty"`

	fqdn          string // fullly hierarchical name to the record.
	childRecDecls []flatfile.RecDecl
}

func (r *RecordDecl) DeclName() string {
	return r.Name
}

func (r *RecordDecl) Target() bool {
	return r.IsTarget
}

func (r *RecordDecl) Group() bool {
	return r.Type != nil && *r.Type == typeGroup
}

// min defaults to 0
func (r *RecordDecl) MinOccurs() int {
	switch r.Min {
	case nil:
		return 0
	default:
		return *r.Min
	}
}

// max defaults to -1, aka unbounded
func (r *RecordDecl) MaxOccurs() int {
	switch {
	case r.Max == nil:
		fallthrough
	case *r.Max < 0:
		return maths.MaxIntValue
	default:
		return *r.Max
	}
}

func (r *RecordDecl) ChildRecDecls() []flatfile.RecDecl {
	return r.childRecDecls
}

func toFlatFileRecDecls(rs []*RecordDecl) []flatfile.RecDecl {
	if len(rs) == 0 {
		return nil
	}
	ret := make([]flatfile.RecDecl, len(rs))
	for i, r := range rs {
		ret[i] = r
	}
	return ret
}

// FileDecl describes CSV specific schema settings for omniparser reader.
type FileDecl struct {
	Delimiter           string        `json:"delimiter,omitempty"`
	ReplaceDoubleQuotes bool          `json:"replace_double_quotes,omitempty"`
	Records             []*RecordDecl `json:"records,omitempty"`
}
