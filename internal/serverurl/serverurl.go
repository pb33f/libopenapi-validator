// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package serverurl reads OpenAPI server URLs, which may be templates that url.Parse rejects.
package serverurl

import (
	"net/url"
	"strings"
)

// BasePath returns the path of an OpenAPI server URL, as url.Parse reports it.
//
// A server variable in the scheme, host, or port (for example "https://{host}/api/v1") makes
// url.Parse fail, so the path is then read from after the host instead.
func BasePath(serverURL string) string {
	u, err := url.Parse(serverURL)
	if err == nil {
		return u.Path
	}
	// drop the scheme separator, then split at the first slash after the host
	_, serverPath, found := strings.Cut(strings.Replace(serverURL, "//", "", 1), "/")
	if !found {
		return ""
	}
	if !strings.HasPrefix(serverPath, "/") {
		serverPath = "/" + serverPath
	}
	if u, err = url.Parse(serverPath); err != nil {
		return ""
	}
	return u.Path
}
