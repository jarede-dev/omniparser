package flatfile

import (
	"github.com/jf-tech/omniparser/idr"
)

// RecReader defines how a specific flatfile format should read/match a raw record from
// input stream and convert into an IDR node.
type RecReader interface {
	// MoreUnprocessedData tells if there is any unprocessed data left in the input stream.
	// Possible return values:
	// - nil: more data is available.
	// - io.EOF: no more data is available.
	// - non-nil err: some other (most likely fatal) IO error.
	// Also once a call to this method returns io.EOF, all future calls to it also should
	// return io.EOF.
	MoreUnprocessedData() error
	// ReadRec reads any unprocessed data from input stream and try to match it with the
	// decl passed in. If matched, then return an IDR node; if not, return nil.
	// Implementation notes:
	// - If io.EOF is encountered while there is still unmatched thus unprocessed data,
	//   io.EOF shouldn't be returned.
	// - If the decl is of a Group(), the matching should be using the recursive algorithm
	//   to match the first-in-line non-group descendent decl. If matched, the returned IDR
	//   node should be for this group node, and the actual matched record data should be
	//   internally cached for the next call(s).
	ReadRec(decl RecDecl) (*idr.Node, error)
}
