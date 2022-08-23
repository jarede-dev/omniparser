package fixedlength

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/jf-tech/go-corelib/maths"

	"github.com/jf-tech/omniparser/extensions/omniv21/fileformat/flatfile"
)

// ColumnDecl describes the column structure of an envelope.
type ColumnDecl struct {
	Name        string  `json:"name,omitempty"`
	StartPos    int     `json:"start_pos"` // 1-based. and rune-based.
	Length      int     `json:"length"`    // rune-based length.
	LinePattern *string `json:"line_pattern"`

	linePatternRegexp *regexp.Regexp
}

func (c *ColumnDecl) lineMatch(line []byte) bool {
	if c.linePatternRegexp == nil {
		return true
	}
	return c.linePatternRegexp.Match(line)
}

func (c *ColumnDecl) lineToColumnValue(line []byte) string {
	// StartPos is 1-based and its value >= 1 guaranteed by json schema validation done earlier.
	start := c.StartPos - 1
	// First chop off the prefix prior to c.StartPos
	for start > 0 && len(line) > 0 {
		_, adv := utf8.DecodeRune(line)
		line = line[adv:]
		start--
	}
	// Then from that position, count c.Length runes and that's the string value we need.
	// Note if c.Length is longer than what's left in the line, we'll simply take all of
	// the remaining line (and no error here, since we haven't yet seen a useful case where
	// we need to be this strict.)
	lenCount := c.Length
	i := 0
	for lenCount > 0 && i < len(line) {
		_, adv := utf8.DecodeRune(line[i:])
		i += adv
		lenCount--
	}
	return string(line[:i])
}

const (
	typeEnvelope = "envelope"
	typeGroup    = "envelope_group"
)

// EnvelopeDecl describes an envelope of a fixed-length input.
// if rows/header/footer none specified, then default to rows = 1
// scheam validation guarantees rows/header cannot be specified at the same time.
// footer is optional.
type EnvelopeDecl struct {
	Name     string          `json:"name,omitempty"`
	Rows     *int            `json:"rows,omitempty"`   // must not specify on envelope_group
	Header   *string         `json:"header,omitempty"` // must not specify on envelope_group
	Footer   *string         `json:"footer,omitempty"` // must not specify on envelope_group
	Type     *string         `json:"type,omitempty"`
	IsTarget bool            `json:"is_target,omitempty"`
	Min      *int            `json:"min,omitempty"`
	Max      *int            `json:"max,omitempty"`
	Columns  []*ColumnDecl   `json:"columns,omitempty"`
	Children []*EnvelopeDecl `json:"child_envelopes,omitempty"`

	fqdn          string // fullly hierarchical name to the envelope.
	childRecDecls []flatfile.RecDecl
	headerRegexp  *regexp.Regexp
	footerRegexp  *regexp.Regexp
}

func (e *EnvelopeDecl) Target() bool {
	return e.IsTarget
}

func (e *EnvelopeDecl) Group() bool {
	return e.Type != nil && *e.Type == typeGroup
}

// min defaults to 0
func (e *EnvelopeDecl) MinOccurs() int {
	switch e.Min {
	case nil:
		return 0
	default:
		return *e.Min
	}
}

// max defaults to -1, aka unbounded
func (e *EnvelopeDecl) MaxOccurs() int {
	switch {
	case e.Max == nil:
		fallthrough
	case *e.Max < 0:
		return maths.MaxIntValue
	default:
		return *e.Max
	}
}

func (e *EnvelopeDecl) ChildRecDecls() []flatfile.RecDecl {
	return e.childRecDecls
}

func (e *EnvelopeDecl) rowsBased() bool {
	// if header/footer used, header must be specified; if header is not, then it's rows based.
	return e.Header == nil
}

// rows defaults to 1.
func (e *EnvelopeDecl) rows() int {
	if !e.rowsBased() {
		panic(fmt.Sprintf("envelope '%s' is not rows based", e.fqdn))
	}
	if e.Rows == nil {
		return 1
	}
	return *e.Rows
}

func (e *EnvelopeDecl) matchHeader(line []byte) bool {
	if e.headerRegexp == nil {
		panic(fmt.Sprintf("envelope '%s' is not header/footer based", e.fqdn))
	}
	return e.headerRegexp.Match(line)
}

func (e *EnvelopeDecl) matchFooter(line []byte) bool {
	if e.footerRegexp == nil {
		return true
	}
	return e.footerRegexp.Match(line)
}

func toFlatFileRecDecls(es []*EnvelopeDecl) []flatfile.RecDecl {
	if len(es) == 0 {
		return nil
	}
	ret := make([]flatfile.RecDecl, len(es))
	for i, d := range es {
		ret[i] = d
	}
	return ret
}

// FileDecl describes fixed-length specific schema settings for omniparser reader.
type FileDecl struct {
	Envelopes []*EnvelopeDecl `json:"envelopes,omitempty"`
}
