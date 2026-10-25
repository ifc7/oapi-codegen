package codegen

import (
	"fmt"
	"os"
	"strconv"
)

// goFieldRename describes a struct field whose Go name collided with an
// earlier field's in the same struct and was given a numeric suffix.
type goFieldRename struct {
	index      int    // position of the renamed field
	firstIndex int    // position of the earlier field that kept the name
	name       string // the colliding Go name
	newName    string // the unique name given to the field at index
}

// dedupeGoFieldNames finds Go field names that occur more than once, which
// happens when distinct spec names normalize to the same identifier (e.g.
// "StartTime", "StartTime<" and "StartTime>" all become StartTime). The first
// occurrence keeps its name; each later one gets the smallest numeric suffix,
// starting at 2, that is not already in use.
func dedupeGoFieldNames(names []string) []goFieldRename {
	used := make(map[string]bool, len(names))
	for _, n := range names {
		used[n] = true
	}

	var renames []goFieldRename
	first := make(map[string]int, len(names))
	for i, n := range names {
		firstIndex, seen := first[n]
		if !seen {
			first[n] = i
			continue
		}
		newName := n
		for suffix := 2; used[newName]; suffix++ {
			newName = n + strconv.Itoa(suffix)
		}
		used[newName] = true
		renames = append(renames, goFieldRename{index: i, firstIndex: firstIndex, name: n, newName: newName})
	}
	return renames
}

// warnGoFieldNameCollision reports, once per generation run, that a field of
// the given struct was renamed to avoid a Go name collision.
func warnGoFieldNameCollision(structName, firstSpecName, specName string, r goFieldRename) {
	msg := fmt.Sprintf("Warning: %s: %q and %q both map to Go field name %s; generating %s for %q. Set x-go-name to choose the name.\n",
		structName, firstSpecName, specName, r.name, r.newName, specName)
	if globalState.warnedFieldNameCollisions[msg] {
		return
	}
	if globalState.warnedFieldNameCollisions == nil {
		globalState.warnedFieldNameCollisions = make(map[string]bool)
	}
	globalState.warnedFieldNameCollisions[msg] = true
	fmt.Fprint(os.Stderr, msg)
}

// dedupePropertyGoFieldNames gives each property of a struct a unique Go field
// name, warning about any it had to rename.
func dedupePropertyGoFieldNames(structName string, props []Property) {
	names := make([]string, len(props))
	for i, p := range props {
		names[i] = p.GoFieldName()
	}
	for _, r := range dedupeGoFieldNames(names) {
		props[r.index].goFieldNameOverride = r.newName
		warnGoFieldNameCollision(structName, props[r.firstIndex].JsonFieldName, props[r.index].JsonFieldName, r)
	}
}

// dedupeParamGoNames gives each query, header and cookie parameter of the
// operation a unique Go name, since they share the operation's Params struct.
// The order matches the struct's field order, so the field declared first
// keeps the name.
func (o *OperationDefinition) dedupeParamGoNames() {
	groups := [][]ParameterDefinition{o.QueryParams, o.HeaderParams, o.CookieParams}
	var params []*ParameterDefinition
	for _, g := range groups {
		for i := range g {
			params = append(params, &g[i])
		}
	}
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.GoName()
	}
	for _, r := range dedupeGoFieldNames(names) {
		params[r.index].goNameOverride = r.newName
		warnGoFieldNameCollision(o.OperationId+"Params", params[r.firstIndex].ParamName, params[r.index].ParamName, r)
	}
}
