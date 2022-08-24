package flatfile

import (
	"github.com/jf-tech/omniparser/idr"
)

// RecReader defines how a specific flatfile format should read/match a raw record from
// input stream and convert into an IDR node.
type RecReader interface {
	// MoreUnprocessedData tells if there is any unprocessed data left in the input stream.
	// Possible return values:
	// - true, nil: more data is available.
	// - false, nil: no more data is available.
	// - _, err: some (most likely fatal) IO error has occurred.
	// Implementation notes:
	// - If some data is read in and io.EOF is encountered, true,nil should be returned.
	// - If no data is read in and io.EOF is encountered, false,nil should be returned.
	// - Under no circumstances, io.EOF should be returned.
	// - Once a call to this method returns false,nil, all future calls to it also should
	//   always return false, nil.
	MoreUnprocessedData() (bool, error)
	// TODO
	// ReadRec reads any unprocessed data from input stream and try to match it with the
	// decl passed in. If matched, then return an IDR node; if not, return nil.
	// Implementation notes:
	// - If io.EOF is encountered while there is still unmatched thus unprocessed data,
	//   io.EOF shouldn't be returned.
	// - If the decl is of a Group(), the matching should be using the recursive algorithm
	//   to match the first-in-line non-group descendent decl. If matched, the returned IDR
	//   node should be for this group node, and the actual matched record data should be
	//   internally cached for the next call(s).
	ReadAndMatch(decl RecDecl, createIDR bool) (matched bool, node *idr.Node, err error)
}
