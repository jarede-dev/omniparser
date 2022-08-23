package flatfile

type RecDecl interface {
	Target() bool
	Group() bool
	MinOccurs() int
	MaxOccurs() int
	ChildRecDecls() []RecDecl
}

const (
	rootName = "#root"
)

type rootDecl struct {
	children []RecDecl
}

func (d rootDecl) Target() bool             { return false }
func (d rootDecl) Group() bool              { return true }
func (d rootDecl) MinOccurs() int           { return 1 }
func (d rootDecl) MaxOccurs() int           { return 1 }
func (d rootDecl) ChildRecDecls() []RecDecl { return d.children }
