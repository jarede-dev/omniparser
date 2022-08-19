package fixedlength

import (
	"errors"
	"fmt"

	"github.com/jf-tech/go-corelib/strs"
)

type validateCtx struct {
	seenTarget bool
}

func (ctx *validateCtx) validateFileDecl(fileDecl *FileDecl) error {
	for _, recDecl := range fileDecl.RecDecls {
		if err := ctx.validateRecDecl(recDecl.Name, recDecl); err != nil {
			return err
		}
	}
	if !ctx.seenTarget {
		if len(fileDecl.RecDecls) > 1 {
			return errors.New("missing record/record_group with 'is_target' = true")
		}
		fileDecl.RecDecls[0].IsTarget = true
	}
	return nil
}

func (ctx *validateCtx) validateRecDecl(recFQDN string, recDecl *RecDecl) error {
	recDecl.fqdn = recFQDN
	if recDecl.MinOccurs() > recDecl.MaxOccurs() {
		return fmt.Errorf("record '%s' has 'min' value %d > 'max' value %d",
			recFQDN, recDecl.MinOccurs(), recDecl.MaxOccurs())
	}
	if recDecl.IsTarget {
		if ctx.seenTarget {
			return fmt.Errorf(
				"a second record/group ('%s') with 'is_target' = true is not allowed", recFQDN)
		}
		ctx.seenTarget = true
	}
	if recDecl.Group() {
		if len(recDecl.Children) <= 0 {
			return fmt.Errorf(
				"record_group '%s' must have at least one child record/record_group", recFQDN)
		}
		if len(recDecl.Fields) > 0 {
			return fmt.Errorf("record_group '%s' must not any fields", recFQDN)
		}
	}
	for _, child := range recDecl.Children {
		err := ctx.validateRecDecl(strs.BuildFQDN2(fqdnDelim, recFQDN, child.Name), child)
		if err != nil {
			return err
		}
	}
	recDecl.childDecls = toFlatFileRecDecls(recDecl.Children)
	return nil
}
