package model

import (
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// sortedKeys returns a map's keys in sorted order, matching kin-openapi's
// componentNames. Keys of Paths, Responses and Callback are sorted, not
// insertion-ordered.
func sortedKeys[E any](s map[string]E) []string {
	out := make([]string, 0, len(s))
	for key := range s {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

// Paths is an OpenAPI Paths Object. The entries live in an unexported map,
// as they do in kin-openapi, so Map returns a copy and iteration order is
// not part of the value.
type Paths struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	m map[string]*PathItem
}

// NewPaths builds an empty Paths object.
func NewPaths() *Paths {
	return &Paths{m: make(map[string]*PathItem)}
}

// Keys returns the path keys, sorted.
func (paths *Paths) Keys() []string {
	return sortedKeys(paths.Map())
}

// Value returns the path item for key, or nil.
func (paths *Paths) Value(key string) *PathItem {
	if paths.Len() == 0 {
		return nil
	}
	return paths.m[key]
}

// Set adds or replaces key. paths must be non-nil.
func (paths *Paths) Set(key string, value *PathItem) {
	if paths.m == nil {
		paths.m = make(map[string]*PathItem)
	}
	paths.m[key] = value
}

// Len returns the number of paths, not counting extensions.
func (paths *Paths) Len() int {
	if paths == nil || paths.m == nil {
		return 0
	}
	return len(paths.m)
}

// Delete removes key. A nil receiver is a no-op.
func (paths *Paths) Delete(key string) {
	if paths != nil && paths.m != nil {
		delete(paths.m, key)
	}
}

// Map returns a copy of the paths. The copy is empty, never nil, when paths
// is nil or has no entries. Iteration order is not significant.
func (paths *Paths) Map() map[string]*PathItem {
	if paths == nil || len(paths.m) == 0 {
		return make(map[string]*PathItem)
	}
	m := make(map[string]*PathItem, len(paths.m))
	maps.Copy(m, paths.m)
	return m
}

// Find returns the path item whose template matches key.
//
// A direct key wins. Otherwise the match ignores template variable names
// (a "{name*}" wildcard still matches only the same normalized pattern).
// For example Find("/person/{name}") returns the item stored at
// "/person/{personName}".
func (paths *Paths) Find(key string) *PathItem {
	if pathItem := paths.Value(key); pathItem != nil {
		return pathItem
	}

	normalizedPath, expected, _ := normalizeTemplatedPath(key)
	pathsMap := paths.Map()
	for _, path := range sortedKeys(pathsMap) {
		pathNormalized, got, _ := normalizeTemplatedPath(path)
		if got == expected && pathNormalized == normalizedPath {
			return pathsMap[path]
		}
	}
	return nil
}

// normalizeTemplatedPath rewrites each {var} to {} and counts the variables.
// A "*" written immediately before a closing brace is kept, matching
// kin-openapi. The third result is the set of variable names.
func normalizeTemplatedPath(path string) (string, uint, map[string]struct{}) {
	if strings.IndexByte(path, '{') < 0 {
		return path, 0, nil
	}

	var buffTpl strings.Builder
	buffTpl.Grow(len(path))

	var (
		cc         rune
		count      uint
		isVariable bool
		vars       = make(map[string]struct{})
		buffVar    strings.Builder
	)
	for i, c := range path {
		if isVariable {
			if c == '}' {
				isVariable = false

				vars[buffVar.String()] = struct{}{}
				buffVar = strings.Builder{}

				// First append a possible '*' before this character.
				// The '}' itself is appended below.
				if i > 0 && cc == '*' {
					buffTpl.WriteRune(cc)
				}
			} else {
				buffVar.WriteRune(c)
				continue
			}
		} else if c == '{' {
			isVariable = true
			count++
		}

		buffTpl.WriteRune(c)
		cc = c
	}
	return buffTpl.String(), count, vars
}

// MethodQuery is the HTTP QUERY method, a fixed Path Item field in OpenAPI 3.2.
const MethodQuery = "QUERY"

// PathItem is an OpenAPI Path Item Object.
type PathItem struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Ref         string     `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Summary     string     `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Connect     *Operation `json:"connect,omitempty" yaml:"connect,omitempty"`
	Delete      *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Get         *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Head        *Operation `json:"head,omitempty" yaml:"head,omitempty"`
	Options     *Operation `json:"options,omitempty" yaml:"options,omitempty"`
	Patch       *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Post        *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put         *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Trace       *Operation `json:"trace,omitempty" yaml:"trace,omitempty"`
	Query       *Operation `json:"query,omitempty" yaml:"query,omitempty"` // OpenAPI >=3.2
	Servers     Servers    `json:"servers,omitempty" yaml:"servers,omitempty"`
	Parameters  Parameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// AdditionalOperations maps custom HTTP methods to operations (OpenAPI 3.2).
	// Keys must not repeat the methods that have their own fields above.
	AdditionalOperations map[string]*Operation `json:"additionalOperations,omitempty" yaml:"additionalOperations,omitempty"`
}

// Operations returns the path item's operations keyed by HTTP method,
// including QUERY and AdditionalOperations. A fixed field wins when an
// additional key collides with it.
func (pathItem *PathItem) Operations() map[string]*Operation {
	operations := pathItem.fixedOperations()
	for method, operation := range pathItem.AdditionalOperations {
		if operation == nil {
			continue
		}
		if _, ok := operations[method]; ok {
			continue
		}
		operations[method] = operation
	}
	return operations
}

func (pathItem *PathItem) fixedOperations() map[string]*Operation {
	operations := make(map[string]*Operation)
	if v := pathItem.Connect; v != nil {
		operations[http.MethodConnect] = v
	}
	if v := pathItem.Delete; v != nil {
		operations[http.MethodDelete] = v
	}
	if v := pathItem.Get; v != nil {
		operations[http.MethodGet] = v
	}
	if v := pathItem.Head; v != nil {
		operations[http.MethodHead] = v
	}
	if v := pathItem.Options; v != nil {
		operations[http.MethodOptions] = v
	}
	if v := pathItem.Patch; v != nil {
		operations[http.MethodPatch] = v
	}
	if v := pathItem.Post; v != nil {
		operations[http.MethodPost] = v
	}
	if v := pathItem.Put; v != nil {
		operations[http.MethodPut] = v
	}
	if v := pathItem.Trace; v != nil {
		operations[http.MethodTrace] = v
	}
	if v := pathItem.Query; v != nil {
		operations[MethodQuery] = v
	}
	return operations
}

// GetOperation returns the operation registered for method, or nil.
func (pathItem *PathItem) GetOperation(method string) *Operation {
	switch method {
	case http.MethodConnect:
		return pathItem.Connect
	case http.MethodDelete:
		return pathItem.Delete
	case http.MethodGet:
		return pathItem.Get
	case http.MethodHead:
		return pathItem.Head
	case http.MethodOptions:
		return pathItem.Options
	case http.MethodPatch:
		return pathItem.Patch
	case http.MethodPost:
		return pathItem.Post
	case http.MethodPut:
		return pathItem.Put
	case http.MethodTrace:
		return pathItem.Trace
	case MethodQuery:
		return pathItem.Query
	default:
		return pathItem.AdditionalOperations[method]
	}
}

// SetOperation registers operation under method, replacing any previous one.
// A nil operation clears it. Methods other than the fixed path-item fields
// are stored in AdditionalOperations.
func (pathItem *PathItem) SetOperation(method string, operation *Operation) {
	switch method {
	case http.MethodConnect:
		pathItem.Connect = operation
	case http.MethodDelete:
		pathItem.Delete = operation
	case http.MethodGet:
		pathItem.Get = operation
	case http.MethodHead:
		pathItem.Head = operation
	case http.MethodOptions:
		pathItem.Options = operation
	case http.MethodPatch:
		pathItem.Patch = operation
	case http.MethodPost:
		pathItem.Post = operation
	case http.MethodPut:
		pathItem.Put = operation
	case http.MethodTrace:
		pathItem.Trace = operation
	case MethodQuery:
		pathItem.Query = operation
	default:
		if operation == nil {
			delete(pathItem.AdditionalOperations, method)
			return
		}
		if pathItem.AdditionalOperations == nil {
			pathItem.AdditionalOperations = make(map[string]*Operation, 1)
		}
		pathItem.AdditionalOperations[method] = operation
	}
}

// Operation is an OpenAPI Operation Object.
type Operation struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Tags         []string              `json:"tags,omitempty" yaml:"tags,omitempty"`
	Summary      string                `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description  string                `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID  string                `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Parameters   Parameters            `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody  *RequestBodyRef       `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses    *Responses            `json:"responses" yaml:"responses"`
	Callbacks    Callbacks             `json:"callbacks,omitempty" yaml:"callbacks,omitempty"`
	Deprecated   bool                  `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Security     *SecurityRequirements `json:"security,omitempty" yaml:"security,omitempty"`
	Servers      *Servers              `json:"servers,omitempty" yaml:"servers,omitempty"`
	ExternalDocs *ExternalDocs         `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
}

// Responses is an OpenAPI Responses Object.
type Responses struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	m map[string]*ResponseRef
}

// NewResponses builds an empty Responses object.
func NewResponses() *Responses {
	return &Responses{m: make(map[string]*ResponseRef)}
}

// Keys returns the response keys, sorted.
func (responses *Responses) Keys() []string {
	return sortedKeys(responses.Map())
}

// Value returns the response for key, or nil.
func (responses *Responses) Value(key string) *ResponseRef {
	if responses.Len() == 0 {
		return nil
	}
	return responses.m[key]
}

// Set adds or replaces key. responses must be non-nil.
func (responses *Responses) Set(key string, value *ResponseRef) {
	if responses.m == nil {
		responses.m = make(map[string]*ResponseRef)
	}
	responses.m[key] = value
}

// Len returns the number of responses, not counting extensions.
func (responses *Responses) Len() int {
	if responses == nil || responses.m == nil {
		return 0
	}
	return len(responses.m)
}

// Delete removes key. A nil receiver is a no-op.
func (responses *Responses) Delete(key string) {
	if responses != nil && responses.m != nil {
		delete(responses.m, key)
	}
}

// Map returns a copy of the responses. The copy is empty, never nil, when
// responses is nil or has no entries.
func (responses *Responses) Map() map[string]*ResponseRef {
	if responses == nil || len(responses.m) == 0 {
		return make(map[string]*ResponseRef)
	}
	m := make(map[string]*ResponseRef, len(responses.m))
	maps.Copy(m, responses.m)
	return m
}

// Default returns the default response, or nil.
func (responses *Responses) Default() *ResponseRef {
	return responses.Value("default")
}

// Status returns the response for status. An exact key wins. Otherwise a
// patterned key is used from the status class, such as 201 falling through
// to "2XX". Statuses outside 100–599 have no patterned key.
func (responses *Responses) Status(status int) *ResponseRef {
	st := strconv.FormatInt(int64(status), 10)
	if rref := responses.Value(st); rref != nil {
		return rref
	}
	if 99 < status && status < 600 {
		st = string(st[0]) + "XX"
		switch st {
		case "1XX", "2XX", "3XX", "4XX", "5XX":
			return responses.Value(st)
		}
	}
	return nil
}

// Callbacks are named callbacks, on an operation or in components.
type Callbacks map[string]*CallbackRef

// Callback is an OpenAPI Callback Object, mapping an expression to a path item.
type Callback struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	m map[string]*PathItem
}

// NewCallback builds an empty Callback.
func NewCallback() *Callback {
	return &Callback{m: make(map[string]*PathItem)}
}

// Keys returns the callback expression keys, sorted.
func (callback *Callback) Keys() []string {
	return sortedKeys(callback.Map())
}

// Value returns the path item for key, or nil.
func (callback *Callback) Value(key string) *PathItem {
	if callback.Len() == 0 {
		return nil
	}
	return callback.m[key]
}

// Set adds or replaces key. callback must be non-nil.
func (callback *Callback) Set(key string, value *PathItem) {
	if callback.m == nil {
		callback.m = make(map[string]*PathItem)
	}
	callback.m[key] = value
}

// Len returns the number of callback entries, not counting extensions.
func (callback *Callback) Len() int {
	if callback == nil || callback.m == nil {
		return 0
	}
	return len(callback.m)
}

// Delete removes key. A nil receiver is a no-op.
func (callback *Callback) Delete(key string) {
	if callback != nil && callback.m != nil {
		delete(callback.m, key)
	}
}

// Map returns a copy of the callback entries. The copy is empty, never nil,
// when callback is nil or has no entries.
func (callback *Callback) Map() map[string]*PathItem {
	if callback == nil || len(callback.m) == 0 {
		return make(map[string]*PathItem)
	}
	m := make(map[string]*PathItem, len(callback.m))
	maps.Copy(m, callback.m)
	return m
}
