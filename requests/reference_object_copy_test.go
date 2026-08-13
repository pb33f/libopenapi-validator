// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package requests

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	liberrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/testify/assert"
	"github.com/pb33f/testify/require"
)

func TestReferenceObjectCopiesRequestBodyPerViolation(t *testing.T) {
	const (
		violations = 80
		paddingLen = 256 * 1024
	)

	body := largeObjectWithTypedFields(paddingLen, violations)
	schema := parseSchemaFromSpec(t, objectSchemaRequiringStrings(violations), 3.1)

	valid, errs := ValidateRequestSchema(&ValidateRequestSchemaInput{
		Request: postRequestWithBody(body),
		Schema:  schema,
		Version: 3.1,
	})
	require.False(t, valid)
	require.Len(t, errs, 1)
	require.GreaterOrEqual(t, len(errs[0].SchemaValidationErrors), violations)

	ptrs := uniqueReferenceObjectBackings(errs[0].SchemaValidationErrors)
	t.Logf("body_bytes=%d schema_violations=%d unique_ReferenceObject_backings=%d",
		len(body), len(errs[0].SchemaValidationErrors), len(ptrs))

	assert.Equal(t, 1, len(ptrs))
}

func objectSchemaRequiringStrings(fields int) string {
	var b strings.Builder
	b.WriteString("type: object\nproperties:\n  pad:\n    type: string\n")
	for i := 0; i < fields; i++ {
		fmt.Fprintf(&b, "  f%d:\n    type: string\n", i)
	}
	return b.String()
}

func largeObjectWithTypedFields(paddingLen, fields int) string {
	var b strings.Builder
	b.Grow(paddingLen + fields*16 + 32)
	b.WriteString(`{"pad":"`)
	b.WriteString(strings.Repeat("x", paddingLen))
	b.WriteString(`"`)
	for i := 0; i < fields; i++ {
		fmt.Fprintf(&b, `,"f%d":1`, i)
	}
	b.WriteString(`}`)
	return b.String()
}

func uniqueReferenceObjectBackings(fails []*liberrors.SchemaValidationFailure) map[uintptr]struct{} {
	seen := make(map[uintptr]struct{}, len(fails))
	for _, fail := range fails {
		if fail == nil || fail.ReferenceObject == "" {
			continue
		}
		seen[uintptr(unsafe.Pointer(unsafe.StringData(fail.ReferenceObject)))] = struct{}{}
	}
	return seen
}
