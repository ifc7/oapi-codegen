package model

import (
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestPathsMapIsACopy(t *testing.T) {
	t.Parallel()

	item := &PathItem{Summary: "kept"}
	paths := NewPaths()
	paths.Set("/pets", item)

	copied := paths.Map()
	delete(copied, "/pets")
	require.Same(t, item, paths.Value("/pets"))

	paths.Set("/b", &PathItem{})
	paths.Set("/a", &PathItem{})
	require.Equal(t, []string{"/a", "/b", "/pets"}, paths.Keys())

	var nilPaths *Paths
	require.Nil(t, nilPaths.Value("/pets"))
	require.Empty(t, nilPaths.Map())
	require.Nil(t, nilPaths.Find("/pets"))
	require.Equal(t, 0, nilPaths.Len())
}

func TestPathsFind(t *testing.T) {
	t.Parallel()

	named := &PathItem{Summary: "person"}
	exact := &PathItem{Summary: "exact"}
	paths := NewPaths()
	paths.Set("/person/{personName}", named)
	paths.Set("/files/{name*}", exact)

	require.Same(t, named, paths.Find("/person/{personName}"))
	require.Same(t, named, paths.Find("/person/{name}"))
	require.Nil(t, paths.Find("/person/{name}/extra"))
	require.Same(t, exact, paths.Find("/files/{path*}"))
	require.Nil(t, paths.Find("/missing"))
}

func TestPathsFindMatchesKin(t *testing.T) {
	t.Parallel()

	keys := []string{
		"/person/{personName}",
		"/files/{name*}",
		"/a/{b}/c/{d}",
		"/plain",
		"/{a}{b}",
	}
	probes := append(append([]string{}, keys...),
		"/person/{name}",
		"/files/{path*}",
		"/files/{name}",
		"/a/{x}/c/{y}",
		"/a/{b}/c",
		"/nope",
		"",
	)

	ours := NewPaths()
	kin := openapi3.NewPaths()
	for _, key := range keys {
		ours.Set(key, &PathItem{Summary: key})
		kin.Set(key, &openapi3.PathItem{Summary: key})
	}
	for _, probe := range probes {
		got := ours.Find(probe)
		want := kin.Find(probe)
		if want == nil {
			require.Nil(t, got, "Find %q", probe)
			continue
		}
		require.NotNil(t, got, "Find %q", probe)
		require.Equal(t, want.Summary, got.Summary, "Find %q", probe)
	}
}

func TestPathItemOperations(t *testing.T) {
	t.Parallel()

	get := &Operation{OperationID: "get"}
	query := &Operation{OperationID: "query"}
	custom := &Operation{OperationID: "custom"}
	item := &PathItem{}
	item.SetOperation(http.MethodGet, get)
	item.SetOperation(MethodQuery, query)
	item.SetOperation("PURGE", custom)

	require.Same(t, get, item.GetOperation(http.MethodGet))
	require.Same(t, query, item.GetOperation(MethodQuery))
	require.Same(t, custom, item.GetOperation("PURGE"))
	require.Equal(t, map[string]*Operation{
		http.MethodGet: get,
		MethodQuery:    query,
		"PURGE":        custom,
	}, item.Operations())

	// A fixed field wins over an additional operation of the same method.
	item.AdditionalOperations[http.MethodGet] = &Operation{OperationID: "shadow"}
	require.Same(t, get, item.Operations()[http.MethodGet])

	item.SetOperation(http.MethodGet, nil)
	item.SetOperation("PURGE", nil)
	require.Nil(t, item.Get)
	require.Nil(t, item.GetOperation("PURGE"))
	require.NotContains(t, item.AdditionalOperations, "PURGE")
}

func TestResponsesStatus(t *testing.T) {
	t.Parallel()

	exact := &ResponseRef{Ref: "exact"}
	pattern := &ResponseRef{Ref: "pattern"}
	fallback := &ResponseRef{Ref: "default"}
	responses := NewResponses()
	responses.Set("201", exact)
	responses.Set("2XX", pattern)
	responses.Set("default", fallback)

	require.Same(t, exact, responses.Status(201))
	require.Same(t, pattern, responses.Status(204))
	require.Same(t, pattern, responses.Status(299))
	require.Nil(t, responses.Status(99))
	require.Nil(t, responses.Status(600))
	require.Nil(t, responses.Status(404))
	require.Same(t, fallback, responses.Default())
	require.Equal(t, []string{"201", "2XX", "default"}, responses.Keys())

	var nilResponses *Responses
	require.Nil(t, nilResponses.Status(200))
	require.Nil(t, nilResponses.Default())
}

func TestCallbackKeysAreSorted(t *testing.T) {
	t.Parallel()

	callback := NewCallback()
	callback.Set("{$request.body#/id}", &PathItem{Summary: "id"})
	callback.Set("{$request.body#/name}", &PathItem{Summary: "name"})
	require.Equal(t, []string{"{$request.body#/id}", "{$request.body#/name}"}, callback.Keys())
	require.Equal(t, "name", callback.Value("{$request.body#/name}").Summary)

	copied := callback.Map()
	delete(copied, "{$request.body#/id}")
	require.Equal(t, 2, callback.Len())
}
