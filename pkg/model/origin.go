package model

// Origin is where a collection was written in its source document.
// Key is the collection itself. Fields holds each scalar field. Sequences
// holds each item of a sequence-valued field, keyed by the field name.
//
// The shape matches kin-openapi's Origin, including source positions the
// generator reads (route order uses Key.Line).
type Origin struct {
	Key       *Location             `json:"key,omitempty" yaml:"key,omitempty"`
	Fields    FieldLocations        `json:"fields,omitempty" yaml:"fields,omitempty"`
	Sequences map[string][]Location `json:"sequences,omitempty" yaml:"sequences,omitempty"`
}

// FieldLocations holds the locations of a collection's scalar fields, in
// document order. It is a slice rather than a map, matching kin-openapi:
// each Location already carries its Name.
type FieldLocations []Location

// Get returns the location of the named field, or the zero Location when
// the field has none. Use Lookup to tell an absent field from a zero location.
func (f FieldLocations) Get(name string) Location {
	loc, _ := f.Lookup(name)
	return loc
}

// Lookup returns the location of the named field and whether it was found.
func (f FieldLocations) Lookup(name string) (Location, bool) {
	for i := range f {
		if f[i].Name == name {
			return f[i], true
		}
	}
	return Location{}, false
}

// Location is the source position of a field.
type Location struct {
	File   string `json:"file,omitempty" yaml:"file,omitempty"`
	Line   int    `json:"line,omitempty" yaml:"line,omitempty"`
	Column int    `json:"column,omitempty" yaml:"column,omitempty"`
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`

	// EndLine and EndColumn mark the end of the block this location heads
	// (set only on Origin.Key). Both are zero when the source carried no end.
	EndLine   int `json:"endLine,omitempty" yaml:"endLine,omitempty"`
	EndColumn int `json:"endColumn,omitempty" yaml:"endColumn,omitempty"`
}
