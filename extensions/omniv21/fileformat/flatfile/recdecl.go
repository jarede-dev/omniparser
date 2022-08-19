package flatfile

import (
	"github.com/jf-tech/omniparser/errs"
	"github.com/jf-tech/omniparser/idr"
)

type RecDecl interface {
	UniqueName() string
	Target() bool
	Group() bool
	MinOccurs() int
	MaxOccurs() int
	ChildDecls() []RecDecl
	Match(raw interface{}) bool
	ToNode(raw interface{}) (*idr.Node, error)
}

type rootRecDecl struct {
	childDecls []RecDecl
}

func (d rootRecDecl) UniqueName() string     { return "#root" }
func (d rootRecDecl) Target() bool           { return false }
func (d rootRecDecl) Group() bool            { return true }
func (d rootRecDecl) MinOccurs() int         { return 1 }
func (d rootRecDecl) MaxOccurs() int         { return 1 }
func (d rootRecDecl) ChildDecls() []RecDecl  { return d.childDecls }
func (d rootRecDecl) Match(interface{}) bool { return false } // we never run xpath match on root.
func (d rootRecDecl) ToNode(interface{}) (*idr.Node, error) {
	return idr.CreateNode(idr.DocumentNode, d.UniqueName()), nil
}

type RawRecReader interface {
	Cur() (interface{}, error)
	MarkCurDone()
	errs.CtxAwareErr
}
