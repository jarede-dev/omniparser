package fixedlengthadv

// FileDecl describes fixed-length-adv specific schema settings for omniparser reader.
type FileDecl struct {
	RecDecls []*RecDecl `json:"record_declarations,omitempty"`
}
