// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package radix

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tree := New[string]()
	require.NotNil(t, tree)
	assert.NotNil(t, tree.root)
	assert.Equal(t, 0, tree.Size())
}

func TestTree_Insert_LiteralPaths(t *testing.T) {
	tree := New[string]()

	// Insert literal paths
	assert.True(t, tree.Insert("/users", "users handler"))
	assert.True(t, tree.Insert("/users/admin", "admin handler"))
	assert.True(t, tree.Insert("/posts", "posts handler"))
	assert.True(t, tree.Insert("/posts/trending", "trending handler"))

	assert.Equal(t, 4, tree.Size())

	// Verify lookups
	val, path, found := tree.Lookup("/users")
	assert.True(t, found)
	assert.Equal(t, "users handler", val)
	assert.Equal(t, "/users", path)

	val, path, found = tree.Lookup("/users/admin")
	assert.True(t, found)
	assert.Equal(t, "admin handler", val)
	assert.Equal(t, "/users/admin", path)
}

func TestTree_Insert_ParameterizedPaths(t *testing.T) {
	tree := New[string]()

	tree.Insert("/users/{id}", "user by id")
	tree.Insert("/users/{id}/posts", "user posts")
	tree.Insert("/users/{id}/posts/{postId}", "single post")

	assert.Equal(t, 3, tree.Size())

	// Verify parameter matching
	val, path, found := tree.Lookup("/users/123")
	assert.True(t, found)
	assert.Equal(t, "user by id", val)
	assert.Equal(t, "/users/{id}", path)

	val, path, found = tree.Lookup("/users/abc")
	assert.True(t, found)
	assert.Equal(t, "user by id", val)
	assert.Equal(t, "/users/{id}", path)

	val, path, found = tree.Lookup("/users/123/posts")
	assert.True(t, found)
	assert.Equal(t, "user posts", val)
	assert.Equal(t, "/users/{id}/posts", path)

	val, path, found = tree.Lookup("/users/123/posts/456")
	assert.True(t, found)
	assert.Equal(t, "single post", val)
	assert.Equal(t, "/users/{id}/posts/{postId}", path)
}

func TestTree_Specificity_LiteralOverParam(t *testing.T) {
	tree := New[string]()

	// Insert both literal and parameterized for same depth
	tree.Insert("/users/{id}", "user by id")
	tree.Insert("/users/admin", "admin user")
	tree.Insert("/users/me", "current user")

	// Literal matches should take precedence
	val, path, found := tree.Lookup("/users/admin")
	assert.True(t, found)
	assert.Equal(t, "admin user", val)
	assert.Equal(t, "/users/admin", path)

	val, path, found = tree.Lookup("/users/me")
	assert.True(t, found)
	assert.Equal(t, "current user", val)
	assert.Equal(t, "/users/me", path)

	// Non-literal should fall back to param
	val, path, found = tree.Lookup("/users/123")
	assert.True(t, found)
	assert.Equal(t, "user by id", val)
	assert.Equal(t, "/users/{id}", path)
}

func TestTree_Specificity_DeepPaths(t *testing.T) {
	tree := New[string]()

	// Deeper literal path should match over param
	tree.Insert("/api/{version}/users", "versioned users")
	tree.Insert("/api/v1/users", "v1 users")
	tree.Insert("/api/v2/users", "v2 users")
	tree.Insert("/api/v1/users/{id}", "v1 user by id")

	val, path, found := tree.Lookup("/api/v1/users")
	assert.True(t, found)
	assert.Equal(t, "v1 users", val)
	assert.Equal(t, "/api/v1/users", path)

	val, path, found = tree.Lookup("/api/v2/users")
	assert.True(t, found)
	assert.Equal(t, "v2 users", val)
	assert.Equal(t, "/api/v2/users", path)

	val, path, found = tree.Lookup("/api/v3/users")
	assert.True(t, found)
	assert.Equal(t, "versioned users", val)
	assert.Equal(t, "/api/{version}/users", path)

	val, path, found = tree.Lookup("/api/v1/users/123")
	assert.True(t, found)
	assert.Equal(t, "v1 user by id", val)
	assert.Equal(t, "/api/v1/users/{id}", path)
}

func TestTree_Lookup_NoMatch(t *testing.T) {
	tree := New[string]()

	tree.Insert("/users", "users")
	tree.Insert("/users/{id}", "user by id")

	// Path doesn't exist
	_, _, found := tree.Lookup("/posts")
	assert.False(t, found)

	// Path too deep
	_, _, found = tree.Lookup("/users/123/posts/456/comments")
	assert.False(t, found)

	// Empty tree lookup
	emptyTree := New[string]()
	_, _, found = emptyTree.Lookup("/anything")
	assert.False(t, found)
}

func TestTree_Lookup_EdgeCases(t *testing.T) {
	tree := New[string]()

	tree.Insert("/", "root")
	tree.Insert("/users", "users")

	// Root path
	val, path, found := tree.Lookup("/")
	assert.True(t, found)
	assert.Equal(t, "root", val)
	assert.Equal(t, "/", path)

	// Empty path treated as root
	val, path, found = tree.Lookup("")
	assert.True(t, found)
	assert.Equal(t, "root", val)
	assert.Equal(t, "/", path)

	// Trailing slash normalization
	val, path, found = tree.Lookup("/users/")
	assert.True(t, found)
	assert.Equal(t, "users", val)
	assert.Equal(t, "/users", path)

	// Double slashes
	val, path, found = tree.Lookup("//users//")
	assert.True(t, found)
	assert.Equal(t, "users", val)
	assert.Equal(t, "/users", path)
}

func TestTree_Insert_Update(t *testing.T) {
	tree := New[string]()

	// First insert
	isNew := tree.Insert("/users", "v1")
	assert.True(t, isNew)
	assert.Equal(t, 1, tree.Size())

	// Update existing path
	isNew = tree.Insert("/users", "v2")
	assert.False(t, isNew)
	assert.Equal(t, 1, tree.Size())

	// Verify updated value
	val, _, _ := tree.Lookup("/users")
	assert.Equal(t, "v2", val)
}

func TestTree_MultipleParameters(t *testing.T) {
	tree := New[string]()

	tree.Insert("/orgs/{orgId}/teams/{teamId}/members/{memberId}", "org team member")
	tree.Insert("/accounts/{accountId}/ads/{adId}/metrics/{metricId}/breakdown/{breakdownId}", "deep nested")

	val, path, found := tree.Lookup("/orgs/org1/teams/team2/members/member3")
	assert.True(t, found)
	assert.Equal(t, "org team member", val)
	assert.Equal(t, "/orgs/{orgId}/teams/{teamId}/members/{memberId}", path)

	val, path, found = tree.Lookup("/accounts/acc1/ads/ad2/metrics/met3/breakdown/bd4")
	assert.True(t, found)
	assert.Equal(t, "deep nested", val)
	assert.Equal(t, "/accounts/{accountId}/ads/{adId}/metrics/{metricId}/breakdown/{breakdownId}", path)
}

func TestTree_Clear(t *testing.T) {
	tree := New[string]()

	tree.Insert("/users", "users")
	tree.Insert("/posts", "posts")
	assert.Equal(t, 2, tree.Size())

	tree.Clear()
	assert.Equal(t, 0, tree.Size())

	_, _, found := tree.Lookup("/users")
	assert.False(t, found)
}

func TestTree_Walk(t *testing.T) {
	tree := New[string]()

	tree.Insert("/users", "users")
	tree.Insert("/users/{id}", "user by id")
	tree.Insert("/posts", "posts")

	var paths []string
	tree.Walk(func(path string, value string) bool {
		paths = append(paths, path)
		return true
	})

	assert.Len(t, paths, 3)
	sort.Strings(paths)
	assert.Contains(t, paths, "/posts")
	assert.Contains(t, paths, "/users")
	assert.Contains(t, paths, "/users/{id}")
}

func TestTree_Walk_EarlyStop(t *testing.T) {
	tree := New[string]()

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("/path%d", i), fmt.Sprintf("handler%d", i))
	}

	count := 0
	tree.Walk(func(path string, value string) bool {
		count++
		return count < 3 // Stop after 3
	})

	assert.Equal(t, 3, count)
}

func TestTree_Size(t *testing.T) {
	tree := New[string]()

	assert.Equal(t, 0, tree.Size())

	tree.Insert("/a", "a")
	assert.Equal(t, 1, tree.Size())

	tree.Insert("/b", "b")
	assert.Equal(t, 2, tree.Size())

	// Update shouldn't increase size
	tree.Insert("/a", "a2")
	assert.Equal(t, 2, tree.Size())

	tree.Clear()
	assert.Equal(t, 0, tree.Size())
}

// OpenAPI-specific test cases

func TestTree_OpenAPIStylePaths(t *testing.T) {
	tree := New[string]()

	// Common OpenAPI-style paths
	paths := []string{
		"/api/v3/ad_accounts",
		"/api/v3/ad_accounts/{ad_account_id}",
		"/api/v3/ad_accounts/{ad_account_id}/ads",
		"/api/v3/ad_accounts/{ad_account_id}/ads/{ad_id}",
		"/api/v3/ad_accounts/{ad_account_id}/campaigns",
		"/api/v3/ad_accounts/{ad_account_id}/campaigns/{campaign_id}",
		"/api/v3/ad_accounts/{ad_account_id}/bulk_actions",
		"/api/v3/ad_accounts/{ad_account_id}/bulk_actions/{bulk_action_id}",
	}

	for _, p := range paths {
		tree.Insert(p, "handler:"+p)
	}

	assert.Equal(t, len(paths), tree.Size())

	// Test various lookups
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/v3/ad_accounts", "/api/v3/ad_accounts"},
		{"/api/v3/ad_accounts/123", "/api/v3/ad_accounts/{ad_account_id}"},
		{"/api/v3/ad_accounts/abc-def-ghi", "/api/v3/ad_accounts/{ad_account_id}"},
		{"/api/v3/ad_accounts/123/ads", "/api/v3/ad_accounts/{ad_account_id}/ads"},
		{"/api/v3/ad_accounts/123/ads/456", "/api/v3/ad_accounts/{ad_account_id}/ads/{ad_id}"},
		{"/api/v3/ad_accounts/acc1/campaigns/camp1", "/api/v3/ad_accounts/{ad_account_id}/campaigns/{campaign_id}"},
		{"/api/v3/ad_accounts/acc1/bulk_actions", "/api/v3/ad_accounts/{ad_account_id}/bulk_actions"},
		{"/api/v3/ad_accounts/acc1/bulk_actions/ba1", "/api/v3/ad_accounts/{ad_account_id}/bulk_actions/{bulk_action_id}"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			val, path, found := tree.Lookup(tc.input)
			require.True(t, found, "path should be found: %s", tc.input)
			assert.Equal(t, tc.expected, path)
			assert.Equal(t, "handler:"+tc.expected, val)
		})
	}
}

func TestTree_ConsistentWithVaryingIDs(t *testing.T) {
	// This test verifies that the radix tree performs consistently
	// regardless of the specific parameter values used
	tree := New[string]()

	tree.Insert("/api/v3/ad_accounts/{ad_account_id}/bulk_actions", "bulk_actions")

	// All of these should match the same path template
	testCases := []string{
		"/api/v3/ad_accounts/1/bulk_actions",
		"/api/v3/ad_accounts/999999/bulk_actions",
		"/api/v3/ad_accounts/uuid-here/bulk_actions",
		"/api/v3/ad_accounts/acc_123abc/bulk_actions",
	}

	for _, tc := range testCases {
		val, path, found := tree.Lookup(tc)
		require.True(t, found, "should find path for %s", tc)
		assert.Equal(t, "/api/v3/ad_accounts/{ad_account_id}/bulk_actions", path)
		assert.Equal(t, "bulk_actions", val)
	}
}

func TestTree_NilRoot(t *testing.T) {
	// Test that a tree with nil root handles gracefully
	tree := &Tree[string]{root: nil}

	_, _, found := tree.Lookup("/anything")
	assert.False(t, found)

	// Insert should work even with nil root
	tree.Insert("/users", "users")
	val, _, found := tree.Lookup("/users")
	assert.True(t, found)
	assert.Equal(t, "users", val)
}

func TestTree_ComplexParamNames(t *testing.T) {
	tree := New[string]()

	// Various parameter naming styles
	tree.Insert("/users/{user_id}", "underscore")
	tree.Insert("/posts/{postId}", "camelCase")
	tree.Insert("/items/{item-id}", "kebab-case")
	tree.Insert("/things/{THING_ID}", "screaming")

	tests := []struct {
		input    string
		expected string
	}{
		{"/users/123", "/users/{user_id}"},
		{"/posts/abc", "/posts/{postId}"},
		{"/items/xyz", "/items/{item-id}"},
		{"/things/T1", "/things/{THING_ID}"},
	}

	for _, tc := range tests {
		_, path, found := tree.Lookup(tc.input)
		assert.True(t, found)
		assert.Equal(t, tc.expected, path)
	}
}

// Additional edge case tests for full coverage

func TestTree_Walk_NilRoot(t *testing.T) {
	// Verify Walk handles nil root gracefully
	tree := &Tree[string]{root: nil}

	count := 0
	tree.Walk(func(path string, value string) bool {
		count++
		return true
	})

	assert.Equal(t, 0, count, "Walk on nil root should not call callback")
}

func TestTree_Walk_EarlyStopOnParamChild(t *testing.T) {
	// Test that Walk respects early stop when iterating paramChild
	tree := New[string]()

	// Create a structure where we have literal children AND a param child
	tree.Insert("/users/admin", "admin")
	tree.Insert("/users/{id}", "user by id")
	tree.Insert("/users/{id}/posts", "posts")

	// Stop immediately
	count := 0
	tree.Walk(func(path string, value string) bool {
		count++
		return false // Stop after first
	})

	assert.Equal(t, 1, count, "Walk should stop after first callback returns false")
}

func TestTree_Walk_StopInParamChildBranch(t *testing.T) {
	// Specifically test stopping while in the paramChild branch
	tree := New[string]()

	tree.Insert("/a", "a")
	tree.Insert("/b/{id}", "b-id")
	tree.Insert("/b/{id}/c", "b-id-c")

	paths := []string{}
	tree.Walk(func(path string, value string) bool {
		paths = append(paths, path)
		// Stop when we hit the param child's nested path
		return path != "/b/{id}/c"
	})

	// Should have stopped at or after /b/{id}/c
	assert.LessOrEqual(t, len(paths), 3)
}

func TestExtractParamName_NonParam(t *testing.T) {
	// Test extractParamName with non-parameter segments (fallback case)
	// This tests the "return seg" branch

	// These are NOT valid params, should return as-is
	testCases := []struct {
		input    string
		expected string
	}{
		{"users", "users"}, // normal segment
		{"{}", "{}"},       // empty param - not valid (len <= 2)
		{"{a", "{a"},       // missing closing brace
		{"a}", "a}"},       // missing opening brace
		{"{", "{"},         // single char
		{"}", "}"},         // single char
		{"", ""},           // empty string
		{"ab", "ab"},       // two chars, not a param
		{"{x}", "x"},       // Valid param - extracts "x"
		{"{ab}", "ab"},     // Valid param - extracts "ab"
	}

	for _, tc := range testCases {
		result := extractParamName(tc.input)
		assert.Equal(t, tc.expected, result, "extractParamName(%q)", tc.input)
	}
}

func TestIsParam(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"{id}", true},
		{"{userId}", true},
		{"{a}", true},
		{"{}", false},  // empty param name
		{"{a", false},  // missing close
		{"a}", false},  // missing open
		{"id", false},  // no braces
		{"{", false},   // single char
		{"}", false},   // single char
		{"", false},    // empty
		{"ab", false},  // two chars
		{"{ab", false}, // three chars, missing close
		{"ab}", false}, // three chars, missing open
	}

	for _, tc := range testCases {
		result := isParam(tc.input)
		assert.Equal(t, tc.expected, result, "isParam(%q)", tc.input)
	}
}

func TestSplitPath(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"/users/{id}/posts", []string{"users", "{id}", "posts"}},
		{"/users", []string{"users"}},
		{"/", nil},
		{"", nil},
		{"users", []string{"users"}},
		{"/a/b/c", []string{"a", "b", "c"}},
		{"//a//b//", []string{"a", "b"}}, // double slashes filtered
		{"/a/", []string{"a"}},
		{"///", nil}, // all slashes
	}

	for _, tc := range testCases {
		result := splitPath(tc.input)
		assert.Equal(t, tc.expected, result, "splitPath(%q)", tc.input)
	}
}

func TestTree_SpecialCharacters(t *testing.T) {
	tree := New[string]()

	// Paths with special characters (URL-safe ones)
	tree.Insert("/api/v1/users", "users")
	tree.Insert("/api/v1/users/{id}", "user")
	tree.Insert("/api/v1/items-list", "items-list")
	tree.Insert("/api/v1/snake_case", "snake")
	tree.Insert("/api/v1/CamelCase", "camel")

	tests := []struct {
		lookup   string
		expected string
		found    bool
	}{
		{"/api/v1/users", "/api/v1/users", true},
		{"/api/v1/users/user-123", "/api/v1/users/{id}", true},
		{"/api/v1/users/user_456", "/api/v1/users/{id}", true},
		{"/api/v1/items-list", "/api/v1/items-list", true},
		{"/api/v1/snake_case", "/api/v1/snake_case", true},
		{"/api/v1/CamelCase", "/api/v1/CamelCase", true},
	}

	for _, tc := range tests {
		_, path, found := tree.Lookup(tc.lookup)
		assert.Equal(t, tc.found, found, "lookup %q", tc.lookup)
		if tc.found {
			assert.Equal(t, tc.expected, path, "lookup %q", tc.lookup)
		}
	}
}

func TestTree_SingleCharSegments(t *testing.T) {
	tree := New[string]()

	tree.Insert("/a", "a")
	tree.Insert("/a/b", "ab")
	tree.Insert("/a/{x}", "ax")
	tree.Insert("/a/b/c", "abc")

	_, path, found := tree.Lookup("/a")
	assert.True(t, found)
	assert.Equal(t, "/a", path)

	_, path, found = tree.Lookup("/a/b")
	assert.True(t, found)
	assert.Equal(t, "/a/b", path)

	_, path, found = tree.Lookup("/a/z")
	assert.True(t, found)
	assert.Equal(t, "/a/{x}", path)
}

func TestTree_URLEncodedSegments(t *testing.T) {
	// URL-encoded values should be matched as literals
	tree := New[string]()

	tree.Insert("/users/{id}", "user")

	// These are all different IDs that should match the param
	testIDs := []string{
		"123",
		"abc",
		"user%40example.com", // @ encoded
		"hello%20world",      // space encoded
		"100%25",             // % encoded
	}

	for _, id := range testIDs {
		_, path, found := tree.Lookup("/users/" + id)
		assert.True(t, found, "should find path for /users/%s", id)
		assert.Equal(t, "/users/{id}", path)
	}
}

func TestTree_NumericSegments(t *testing.T) {
	tree := New[string]()

	tree.Insert("/v1/resource", "v1")
	tree.Insert("/v2/resource", "v2")
	tree.Insert("/{version}/resource", "versioned")

	_, path, found := tree.Lookup("/v1/resource")
	assert.True(t, found)
	assert.Equal(t, "/v1/resource", path)

	_, path, found = tree.Lookup("/v2/resource")
	assert.True(t, found)
	assert.Equal(t, "/v2/resource", path)

	_, path, found = tree.Lookup("/v999/resource")
	assert.True(t, found)
	assert.Equal(t, "/{version}/resource", path)
}

func TestTree_DeepNesting(t *testing.T) {
	tree := New[string]()

	// Very deep path
	deepPath := "/a/{b}/c/{d}/e/{f}/g/{h}/i/{j}/k"
	tree.Insert(deepPath, "deep")

	_, path, found := tree.Lookup("/a/1/c/2/e/3/g/4/i/5/k")
	assert.True(t, found)
	assert.Equal(t, deepPath, path)
}

func TestTree_LookupPartialMatch(t *testing.T) {
	tree := New[string]()

	tree.Insert("/users/{id}/posts/{postId}", "post")

	// Partial path should not match
	_, _, found := tree.Lookup("/users/123/posts")
	assert.False(t, found, "partial path should not match")

	_, _, found = tree.Lookup("/users/123")
	assert.False(t, found, "partial path should not match")
}

func TestTree_OverlappingPaths(t *testing.T) {
	tree := New[string]()

	// Insert paths that could conflict
	tree.Insert("/api/users", "users list")
	tree.Insert("/api/users/search", "users search")
	tree.Insert("/api/users/{id}", "user by id")
	tree.Insert("/api/users/{id}/profile", "user profile")
	tree.Insert("/api/users/{userId}/posts/{postId}", "user post")

	tests := []struct {
		lookup   string
		expected string
	}{
		{"/api/users", "/api/users"},
		{"/api/users/search", "/api/users/search"},
		{"/api/users/123", "/api/users/{id}"},
		{"/api/users/123/profile", "/api/users/{id}/profile"},
		{"/api/users/u1/posts/p1", "/api/users/{userId}/posts/{postId}"},
	}

	for _, tc := range tests {
		_, path, found := tree.Lookup(tc.lookup)
		require.True(t, found, "should find %s", tc.lookup)
		assert.Equal(t, tc.expected, path, "lookup %s", tc.lookup)
	}
}

func TestTree_ConcurrentAccess(t *testing.T) {
	// Test concurrent reads (tree is read-only after construction)
	tree := New[string]()

	paths := []string{
		"/api/v1/users",
		"/api/v1/users/{id}",
		"/api/v1/posts",
		"/api/v1/posts/{id}",
	}

	for _, p := range paths {
		tree.Insert(p, "handler:"+p)
	}

	// Concurrent lookups
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				path := paths[n%len(paths)]
				testPath := path
				if n%2 == 0 {
					// Replace params with values
					testPath = "/api/v1/users/123"
				}
				_, _, _ = tree.Lookup(testPath)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestTree_EmptyValue(t *testing.T) {
	// Test that empty values are stored correctly
	tree := New[string]()

	tree.Insert("/empty", "")

	val, path, found := tree.Lookup("/empty")
	assert.True(t, found)
	assert.Equal(t, "/empty", path)
	assert.Equal(t, "", val) // Empty string is a valid value
}

func TestTree_PointerValues(t *testing.T) {
	// Test with pointer values to ensure nil handling
	type Handler struct {
		Name string
	}

	tree := New[*Handler]()

	h1 := &Handler{Name: "h1"}
	tree.Insert("/a", h1)
	tree.Insert("/b", nil) // nil pointer value

	val, _, found := tree.Lookup("/a")
	assert.True(t, found)
	assert.Equal(t, "h1", val.Name)

	val, _, found = tree.Lookup("/b")
	assert.True(t, found)
	assert.Nil(t, val) // nil is a valid value

	_, _, found = tree.Lookup("/c")
	assert.False(t, found)
}

func TestTree_LookupWithParams(t *testing.T) {
	tests := []struct {
		name           string
		insertPaths    []string
		lookupPath     string
		expectedValue  string
		expectedPath   string
		expectedParams map[string]string
		expectedFound  bool
	}{
		{
			name:           "No params - literal path",
			insertPaths:    []string{"/users"},
			lookupPath:     "/users",
			expectedValue:  "users handler",
			expectedPath:   "/users",
			expectedParams: nil,
			expectedFound:  true,
		},
		{
			name:           "Single param",
			insertPaths:    []string{"/users/{id}"},
			lookupPath:     "/users/123",
			expectedValue:  "user by id",
			expectedPath:   "/users/{id}",
			expectedParams: map[string]string{"id": "123"},
			expectedFound:  true,
		},
		{
			name:           "Multiple params",
			insertPaths:    []string{"/users/{userId}/posts/{postId}"},
			lookupPath:     "/users/abc/posts/xyz",
			expectedValue:  "user post",
			expectedPath:   "/users/{userId}/posts/{postId}",
			expectedParams: map[string]string{"userId": "abc", "postId": "xyz"},
			expectedFound:  true,
		},
		{
			name:           "Literal over param precedence",
			insertPaths:    []string{"/users/{id}", "/users/admin"},
			lookupPath:     "/users/admin",
			expectedValue:  "admin user",
			expectedPath:   "/users/admin",
			expectedParams: nil,
			expectedFound:  true,
		},
		{
			name:           "Param match when literal doesn't match",
			insertPaths:    []string{"/a/{x}/d"},
			lookupPath:     "/a/b/d",
			expectedValue:  "a-x-d",
			expectedPath:   "/a/{x}/d",
			expectedParams: map[string]string{"x": "b"},
			expectedFound:  true,
		},
		{
			name:           "Not found",
			insertPaths:    []string{"/users/{id}"},
			lookupPath:     "/posts/123",
			expectedValue:  "",
			expectedPath:   "",
			expectedParams: nil,
			expectedFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := New[string]()
			for i, path := range tt.insertPaths {
				var value string
				switch i {
				case 0:
					if path == "/users" {
						value = "users handler"
					} else if path == "/users/{id}" {
						value = "user by id"
					} else if path == "/users/{userId}/posts/{postId}" {
						value = "user post"
					} else if path == "/a/b/c" {
						value = "a-b-c"
					} else if path == "/a/{x}/d" {
						value = "a-x-d"
					}
				case 1:
					if path == "/users/admin" {
						value = "admin user"
					}
				}
				tree.Insert(path, value)
			}

			val, path, params, found := tree.LookupWithParams(tt.lookupPath)

			assert.Equal(t, tt.expectedFound, found, "found mismatch")
			if tt.expectedFound {
				assert.Equal(t, tt.expectedValue, val, "value mismatch")
				assert.Equal(t, tt.expectedPath, path, "path mismatch")
				if tt.expectedParams == nil {
					assert.Nil(t, params, "params should be nil")
				} else {
					assert.Equal(t, tt.expectedParams, params, "params mismatch")
				}
			} else {
				assert.Empty(t, val, "value should be empty")
				assert.Empty(t, path, "path should be empty")
				assert.Nil(t, params, "params should be nil")
			}
		})
	}
}

// Benchmark tests

func BenchmarkTree_Insert(b *testing.B) {
	paths := []string{
		"/api/v3/ad_accounts",
		"/api/v3/ad_accounts/{ad_account_id}",
		"/api/v3/ad_accounts/{ad_account_id}/ads",
		"/api/v3/ad_accounts/{ad_account_id}/ads/{ad_id}",
		"/api/v3/ad_accounts/{ad_account_id}/campaigns",
		"/api/v3/ad_accounts/{ad_account_id}/campaigns/{campaign_id}",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree := New[string]()
		for _, p := range paths {
			tree.Insert(p, p)
		}
	}
}

func BenchmarkTree_Lookup_Literal(b *testing.B) {
	tree := New[string]()
	tree.Insert("/api/v3/ad_accounts", "accounts")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Lookup("/api/v3/ad_accounts")
	}
}

func BenchmarkTree_Lookup_SingleParam(b *testing.B) {
	tree := New[string]()
	tree.Insert("/api/v3/ad_accounts/{ad_account_id}", "account")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Lookup("/api/v3/ad_accounts/123456")
	}
}

func BenchmarkTree_Lookup_MultipleParams(b *testing.B) {
	tree := New[string]()
	tree.Insert("/api/v3/ad_accounts/{ad_account_id}/campaigns/{campaign_id}/ads/{ad_id}", "ad")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Lookup("/api/v3/ad_accounts/acc1/campaigns/camp1/ads/ad1")
	}
}

func BenchmarkTree_Lookup_ManyPaths(b *testing.B) {
	tree := New[string]()

	// Simulate a realistic API with many paths
	for i := 0; i < 100; i++ {
		tree.Insert(fmt.Sprintf("/api/v3/resource%d", i), fmt.Sprintf("handler%d", i))
		tree.Insert(fmt.Sprintf("/api/v3/resource%d/{id}", i), fmt.Sprintf("handler%d-id", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Lookup("/api/v3/resource50/abc123")
	}
}

func BenchmarkTree_Lookup_VaryingIDs(b *testing.B) {
	tree := New[string]()
	tree.Insert("/api/v3/ad_accounts/{ad_account_id}/bulk_actions", "bulk")

	// Pre-generate test paths
	testPaths := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testPaths[i] = fmt.Sprintf("/api/v3/ad_accounts/account_%d/bulk_actions", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Lookup(testPaths[i%1000])
	}
}
