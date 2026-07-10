// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package requeststate

import (
	"context"
	"net/http"

	"github.com/pb33f/libopenapi-validator/router"
)

type routeContextKey struct{}

// Route returns the route scoped to the current high-level validation call.
func Route(request *http.Request) *router.Route {
	if request == nil {
		return nil
	}
	route, _ := request.Context().Value(routeContextKey{}).(*router.Route)
	return route
}

// AttachRoute scopes a resolved route to a request and returns an idempotent restoration function.
func AttachRoute(request *http.Request, route *router.Route) func() {
	if request == nil || route == nil {
		return func() {}
	}
	original := request.Context()
	*request = *request.WithContext(context.WithValue(original, routeContextKey{}, route))
	restored := false
	return func() {
		if restored {
			return
		}
		restored = true
		*request = *request.WithContext(original)
	}
}
