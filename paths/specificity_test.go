// Copyright 2023-2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package paths

import (
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
)

func TestComputeSpecificityScore(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{
			name:     "single literal segment",
			path:     "/pets",
			expected: 1000,
		},
		{
			name:     "single parameter segment",
			path:     "/{id}",
			expected: 1,
		},
		{
			name:     "literal then parameter",
			path:     "/pets/{id}",
			expected: 1001,
		},
		{
			name:     "two literal segments",
			path:     "/pets/mine",
			expected: 2000,
		},
		{
			name:     "two parameter segments",
			path:     "/{tenant}/{id}",
			expected: 2,
		},
		{
			name:     "mixed - param literal param",
			path:     "/{tenant}/users/{id}",
			expected: 1002,
		},
		{
			name:     "three literal segments",
			path:     "/api/v1/users",
			expected: 3000,
		},
		{
			name:     "two literals one param",
			path:     "/api/v1/{resource}",
			expected: 2001,
		},
		{
			name:     "four literals",
			path:     "/api/v1/users/profile",
			expected: 4000,
		},
		{
			name:     "label parameter format",
			path:     "/burgers/{.burgerId}/locate",
			expected: 2001,
		},
		{
			name:     "exploded parameter format",
			path:     "/burgers/{burgerId*}/locate",
			expected: 2001,
		},
		{
			name:     "empty path",
			path:     "/",
			expected: 0,
		},
		{
			name:     "OData style path",
			path:     "/entities('{Entity}')",
			expected: 1,
		},
		{
			name:     "complex OData path",
			path:     "/orders(RelationshipNumber='{RelationshipNumber}',ValidityEndDate=datetime'{ValidityEndDate}')",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeSpecificityScore(tt.path)
			assert.Equal(t, tt.expected, score, "path: %s", tt.path)
		})
	}
}

func TestIsParameterSegment(t *testing.T) {
	tests := []struct {
		segment  string
		expected bool
	}{
		{"users", false},
		{"{id}", true},
		{"{.id}", true},
		{"{id*}", true},
		{"mine", false},
		{"", false},
		{"v1", false},
		{"{petId}", true},
		{"{message_id}", true},
		{"Operations", false},
		{"entities('{Entity}')", true},
		{"literal", false},
	}

	for _, tt := range tests {
		t.Run(tt.segment, func(t *testing.T) {
			result := isParameterSegment(tt.segment)
			assert.Equal(t, tt.expected, result, "segment: %s", tt.segment)
		})
	}
}

func TestPathHasMethod(t *testing.T) {
	tests := []struct {
		name     string
		pathItem *v3.PathItem
		method   string
		expected bool
	}{
		{
			name:     "GET exists",
			pathItem: &v3.PathItem{Get: &v3.Operation{}},
			method:   "GET",
			expected: true,
		},
		{
			name:     "GET missing",
			pathItem: &v3.PathItem{Post: &v3.Operation{}},
			method:   "GET",
			expected: false,
		},
		{
			name:     "POST exists",
			pathItem: &v3.PathItem{Post: &v3.Operation{}},
			method:   "POST",
			expected: true,
		},
		{
			name:     "PUT exists",
			pathItem: &v3.PathItem{Put: &v3.Operation{}},
			method:   "PUT",
			expected: true,
		},
		{
			name:     "DELETE exists",
			pathItem: &v3.PathItem{Delete: &v3.Operation{}},
			method:   "DELETE",
			expected: true,
		},
		{
			name:     "OPTIONS exists",
			pathItem: &v3.PathItem{Options: &v3.Operation{}},
			method:   "OPTIONS",
			expected: true,
		},
		{
			name:     "HEAD exists",
			pathItem: &v3.PathItem{Head: &v3.Operation{}},
			method:   "HEAD",
			expected: true,
		},
		{
			name:     "PATCH exists",
			pathItem: &v3.PathItem{Patch: &v3.Operation{}},
			method:   "PATCH",
			expected: true,
		},
		{
			name:     "TRACE exists",
			pathItem: &v3.PathItem{Trace: &v3.Operation{}},
			method:   "TRACE",
			expected: true,
		},
		{
			name:     "unknown method",
			pathItem: &v3.PathItem{Get: &v3.Operation{}},
			method:   "UNKNOWN",
			expected: false,
		},
		{
			name:     "empty pathItem",
			pathItem: &v3.PathItem{},
			method:   "GET",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pathHasMethod(tt.pathItem, tt.method)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSelectMatches(t *testing.T) {
	tests := []struct {
		name               string
		candidates         []pathCandidate
		expectedWithMethod string // expected path for withMethod, or empty if nil
		expectedHighest    string // expected path for highest, or empty if nil
	}{
		{
			name: "single candidate with method",
			candidates: []pathCandidate{
				{path: "/pets/{id}", score: 1001, hasMethod: true},
			},
			expectedWithMethod: "/pets/{id}",
			expectedHighest:    "/pets/{id}",
		},
		{
			name: "single candidate without method",
			candidates: []pathCandidate{
				{path: "/pets/{id}", score: 1001, hasMethod: false},
			},
			expectedWithMethod: "",
			expectedHighest:    "/pets/{id}",
		},
		{
			name: "higher score wins",
			candidates: []pathCandidate{
				{path: "/pets/{id}", score: 1001, hasMethod: true},
				{path: "/pets/mine", score: 2000, hasMethod: true},
			},
			expectedWithMethod: "/pets/mine",
			expectedHighest:    "/pets/mine",
		},
		{
			name: "higher score wins - reverse order",
			candidates: []pathCandidate{
				{path: "/pets/mine", score: 2000, hasMethod: true},
				{path: "/pets/{id}", score: 1001, hasMethod: true},
			},
			expectedWithMethod: "/pets/mine",
			expectedHighest:    "/pets/mine",
		},
		{
			name: "higher score without method is skipped for withMethod",
			candidates: []pathCandidate{
				{path: "/pets/{id}", score: 1001, hasMethod: true},
				{path: "/pets/mine", score: 2000, hasMethod: false},
			},
			expectedWithMethod: "/pets/{id}",
			expectedHighest:    "/pets/mine",
		},
		{
			name: "equal scores - first wins",
			candidates: []pathCandidate{
				{path: "/pets/{petId}", score: 1001, hasMethod: true},
				{path: "/pets/{petName}", score: 1001, hasMethod: true},
			},
			expectedWithMethod: "/pets/{petId}",
			expectedHighest:    "/pets/{petId}",
		},
		{
			name:               "empty candidates",
			candidates:         []pathCandidate{},
			expectedWithMethod: "",
			expectedHighest:    "",
		},
		{
			name: "all candidates without method",
			candidates: []pathCandidate{
				{path: "/pets/{id}", score: 1001, hasMethod: false},
				{path: "/pets/mine", score: 2000, hasMethod: false},
			},
			expectedWithMethod: "",
			expectedHighest:    "/pets/mine",
		},
		{
			name: "three candidates mixed",
			candidates: []pathCandidate{
				{path: "/{tenant}/users/{id}", score: 1002, hasMethod: true},
				{path: "/api/users/{id}", score: 2001, hasMethod: true},
				{path: "/api/users/me", score: 3000, hasMethod: true},
			},
			expectedWithMethod: "/api/users/me",
			expectedHighest:    "/api/users/me",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withMethod, highest := selectMatches(tt.candidates)

			if tt.expectedWithMethod == "" {
				assert.Nil(t, withMethod)
			} else {
				assert.NotNil(t, withMethod)
				assert.Equal(t, tt.expectedWithMethod, withMethod.path)
			}

			if tt.expectedHighest == "" {
				assert.Nil(t, highest)
			} else {
				assert.NotNil(t, highest)
				assert.Equal(t, tt.expectedHighest, highest.path)
			}
		})
	}
}
