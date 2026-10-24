package codegen

// This file holds the generator's plumbing around the allOf merge, such as
// naming a recursive composition. The merge itself is in
// merge_schemas_allof.go.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// inProgressAllOf returns, for a composition that an enclosing frame is
// already generating, a schema that refers to the type that frame is building,
// instead of inlining the body a second time, which is what used to recurse
// until the stack ran out (issue #2542). ok is false when no frame is.
func (ctx genContext) inProgressAllOf(schema *model.Schema, skipOptionalPointer bool) (Schema, bool) {
	frame, ok := ctx.inProgress[schema]
	if !ok {
		return Schema{}, false
	}
	frame.consulted = true
	return Schema{
		GoType:              frame.typeName,
		RefType:             frame.typeName,
		DefineViaAlias:      true,
		SkipOptionalPointer: skipOptionalPointer,
		OAPISchema:          schema,
	}, true
}

// finishAllOf settles the schema a version merged for a composition: it
// remembers the type the composition is an alias of, points OAPISchema back
// at the composition, keeps the description and
// x-go-type-skip-optional-pointer the caller passes, and names the composition
// when something underneath referred back to it (see mergeFrame).
func finishAllOf(ctx genContext, frame *mergeFrame, schema *model.Schema, merged Schema, description string, extensions map[string]any, skipOptionalPointer bool) (Schema, error) {
	// The composition generated as an alias of another type, such as the
	// $ref of `allOf: [$ref X]`: remember that type's schema, which
	// OAPISchema no longer points to, so generatesMarshalJSON can tell that X
	// has a generated MarshalJSON, which a server response envelope has to
	// delegate to.
	if merged.DefineViaAlias && merged.OAPISchema != nil && merged.OAPISchema != schema {
		merged.aliasOf = merged.OAPISchema
	}
	merged.OAPISchema = schema
	// Description is metadata, not a structural constraint, so it doesn't go
	// through the merge (issue #1960).
	if description != "" {
		merged.Description = description
	}
	// x-go-type on the composition returned from generateGoSchema before the
	// merge. x-go-type-skip-optional-pointer only overrides the merged value
	// when the composition sets it itself; otherwise it would clobber the
	// value the merge took from a member that carries the extension (issue
	// #1957).
	if _, ok := extensions[extPropGoTypeSkipOptionalPointer]; ok {
		merged.SkipOptionalPointer = skipOptionalPointer
	}
	// Something underneath referred back to this composition, so it has
	// to resolve to a named type. When nothing did — the overwhelmingly
	// common case — fall through with the anonymous struct this has
	// always produced, byte for byte.
	if frame.consulted {
		switch {
		case merged.RefType == frame.typeName:
			// Already defined under the promised name: generating the
			// merged body hoisted it (generate-types-for-anonymous-schemas).
		case merged.RefType != "":
			// The name handed to the recursive members is not the one the
			// type ended up with, so those references would dangle. Fail
			// loudly rather than emit code that does not compile.
			return Schema{}, fmt.Errorf(
				"recursive allOf composition at %s was generated as %q but its self-references were resolved to %q",
				strings.Join(ctx.nameHint, "."), merged.RefType, frame.typeName)
		case ctx.rootPosition:
			// GenerateTypesForSchemas names this one, from renameSchema
			// rather than from the path, so the name handed to the
			// members above is not the one it will be defined under.
			// Believed unreachable (see genContext.rootPosition); say so
			// rather than emit code that will not compile.
			return Schema{}, fmt.Errorf(
				"recursive allOf composition at the root of %s is not supported: give the composition its own schema",
				strings.Join(ctx.nameHint, "."))
		default:
			typeDef := TypeDefinition{
				TypeName: frame.typeName,
				JsonName: strings.Join(ctx.nameHint, "."),
				Schema:   merged,
			}
			merged.AdditionalTypes = append(merged.AdditionalTypes, typeDef)
			merged.RefType = frame.typeName
		}
	}
	return merged, nil
}

// nonNullTypes returns a type array's entries other than "null".
func nonNullTypes(t *model.Types) []string {
	var out []string
	for _, typ := range t.Slice() {
		if typ != model.TypeNull {
			out = append(out, typ)
		}
	}
	return out
}

// sameSchema reports whether two schema positions describe the same schema:
// the same $ref, or inline schemas with the same content. Source-location
// metadata is not part of the JSON encoding, so two identical
// schemas declared in different places compare equal.
func sameSchema(r1, r2 *model.SchemaRef) bool {
	if r1.Ref != "" || r2.Ref != "" {
		return r1.Ref == r2.Ref
	}
	b1, err1 := json.Marshal(r1.Value)
	b2, err2 := json.Marshal(r2.Value)
	return err1 == nil && err2 == nil && bytes.Equal(b1, b2)
}

// sameTypeSet reports whether two type lists name the same types, in any
// order.
func sameTypeSet(t1, t2 []string) bool {
	if len(t1) != len(t2) {
		return false
	}
	for _, typ := range t1 {
		if !slices.Contains(t2, typ) {
			return false
		}
	}
	return true
}
