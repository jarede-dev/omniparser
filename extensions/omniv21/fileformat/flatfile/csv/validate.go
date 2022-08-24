package csv

import (
	"fmt"

	"github.com/jf-tech/go-corelib/caches"
	"github.com/jf-tech/go-corelib/strs"
)

type validateCtx struct {
	seenTarget bool
}

func (ctx *validateCtx) validateFileDecl(fileDecl *FileDecl) error {
	for _, recordDecl := range fileDecl.Records {
		if err := ctx.validateRecordDecl(recordDecl.Name, recordDecl); err != nil {
			return err
		}
	}
	if !ctx.seenTarget && len(fileDecl.Records) > 0 {
		// for easy of use and convenience, if no is_target=true record is specified, then
		// the first one on the root will be automatically designated as target record.
		fileDecl.Records[0].IsTarget = true
	}
	return nil
}

func (ctx *validateCtx) validateRecordDecl(fqdn string, recordDecl *RecordDecl) (err error) {
	recordDecl.fqdn = fqdn
	if recordDecl.Group() {
		if len(recordDecl.Children) <= 0 {
			return fmt.Errorf(
				"record_group '%s' must have at least one child record/record_group", fqdn)
		}
		if len(recordDecl.Columns) > 0 {
			return fmt.Errorf("record_group '%s' must not any columns", fqdn)
		}
	}
	if recordDecl.Target() {
		if ctx.seenTarget {
			return fmt.Errorf(
				"a second record/record_group ('%s') with 'is_target' = true is not allowed",
				fqdn)
		}
		ctx.seenTarget = true
	}
	if recordDecl.MinOccurs() > recordDecl.MaxOccurs() {
		return fmt.Errorf("record/record_group '%s' has 'min' value %d > 'max' value %d",
			fqdn, recordDecl.MinOccurs(), recordDecl.MaxOccurs())
	}
	for i, colDecl := range recordDecl.Columns {
		err = ctx.validateColumnDecl(fqdn, colDecl, i)
		if err != nil {
			return err
		}
	}
	for _, child := range recordDecl.Children {
		err = ctx.validateRecordDecl(strs.BuildFQDN2("/", fqdn, child.Name), child)
		if err != nil {
			return err
		}
	}
	recordDecl.childRecDecls = toFlatFileRecDecls(recordDecl.Children)
	return nil
}

func (ctx *validateCtx) validateColumnDecl(
	fqdn string, colDecl *ColumnDecl, colIdx int) (err error) {
	if colDecl.Match != nil {
		colDecl.matchRegexp, err = caches.GetRegex(*colDecl.Match)
		if err != nil {
			return fmt.Errorf(
				"record '%s' column[%d/'%s'] has an invalid 'match' regexp '%s': %s",
				fqdn, colIdx+1, colDecl.name(), *colDecl.Match, err.Error())
		}
	}
	return nil
}
