package fixedlengthadv

// ErrInvalidFixedLengthAdv indicates the Fixed-Length-Adv content is corrupted. This is a fatal, non-continuable error.
type ErrInvalidFixedLengthAdv string

func (e ErrInvalidFixedLengthAdv) Error() string { return string(e) }

// IsErrInvalidFixedLengthAdv checks if the `err` is of ErrInvalidFixedLengthAdv type.
func IsErrInvalidFixedLengthAdv(err error) bool {
	switch err.(type) {
	case ErrInvalidFixedLengthAdv:
		return true
	default:
		return false
	}
}

// RawRec represents a raw record.
type RawRec struct {
	valid bool   // only for internal use.
	Name  string // name of the record, e.g. 'HDR', 'NWR', etc.
	Raw   []byte // the raw data of the entire record. not owned, no mod!
}

func resetRawRec(raw *RawRec) {
	raw.valid = false
	raw.Name = ""
	raw.Raw = nil
}
