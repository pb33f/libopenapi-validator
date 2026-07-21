// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

// Package openapi_schemas contains legacy OpenAPI 3.0 and 3.1 schema loaders.
// Document validation uses the embedded 3.0, 3.1, and 3.2 schemas selected by libopenapi.
// fork of the official OpenAPI repo specifications. Using an MD5 hash, we can compare the local version against
// the remote version and determine if they differ, if they do - load the remote version.
package openapi_schemas

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"

	_ "embed"
)

const (
	defaultSchema30URL = "https://raw.githubusercontent.com/pb33f/openapi-specification/main/schemas/v3.0/schema.json"
	defaultSchema31URL = "https://raw.githubusercontent.com/pb33f/openapi-specification/main/schemas/v3.1/schema.json"
)

var (
	schema30, schema31       string
	schema30URL, schema31URL = defaultSchema30URL, defaultSchema31URL
)

// LoadSchema3_0 loads the latest OpenAPI 3.0 specification. The latest version is fetched from the OpenAPI repo.
// and if there is no change in the schema, the local version is returned, otherwise the remote version is returned.
func LoadSchema3_0(schema string) string {
	if schema30 != "" {
		return schema30
	}
	schema30 = extractSchema(schema30URL, schema)
	return schema30
}

// LoadSchema3_1 loads the latest OpenAPI 3.1 specification. The latest version is fetched from the OpenAPI repo.
// and if there is no change in the schema, the local version is returned, otherwise the remote version is returned.
func LoadSchema3_1(schema string) string {
	if schema31 != "" {
		return schema31
	}
	schema31 = extractSchema(schema31URL, schema)
	return schema31
}

func getFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

func extractSchema(url string, local string) string {
	// check the local version against the latest version held in our repo.
	remoteVersion, err := getFile(url)
	if err != nil {
		return local
	}
	remoteHash := md5.Sum(remoteVersion)
	remoteMD5 := hex.EncodeToString(remoteHash[:])

	localHash := md5.Sum([]byte(local))
	localMD5 := hex.EncodeToString(localHash[:])

	if remoteMD5 != localMD5 {
		return string(remoteVersion)
	}
	return local
}
