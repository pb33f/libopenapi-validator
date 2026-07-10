// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package requeststate

import (
	"context"
	"net/http"
	"testing"

	"github.com/pb33f/testify/assert"

	"github.com/pb33f/libopenapi-validator/router"
)

func TestAttachRouteScopesAndRestoresRequestContext(t *testing.T) {
	type contextKey struct{}
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/items", nil)
	original := context.WithValue(request.Context(), contextKey{}, "original")
	*request = *request.WithContext(original)
	first := &router.Route{Path: "/items"}
	second := &router.Route{Path: "/other"}

	restoreFirst := AttachRoute(request, first)
	assert.Same(t, first, Route(request))
	restoreSecond := AttachRoute(request, second)
	assert.Same(t, second, Route(request))
	restoreSecond()
	assert.Same(t, first, Route(request))
	restoreFirst()
	restoreFirst()
	assert.Nil(t, Route(request))
	assert.Equal(t, "original", request.Context().Value(contextKey{}))

	AttachRoute(nil, first)()
	AttachRoute(request, nil)()
	assert.Nil(t, Route(nil))
}
