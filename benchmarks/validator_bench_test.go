// Copyright 2025 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT
//
// Comprehensive benchmarks for libopenapi-validator performance analysis.
// These benchmarks cover the full validation pipeline with production-like workloads
// modeled after the Reddit Ads API BulkActions endpoint.
//
// Run with:
//   go test -bench=. -benchmem -count=5 -timeout=30m ./benchmarks/ | tee benchmark_results.txt
//
// For CPU profiling:
//   go test -bench=BenchmarkFullValidation -cpuprofile=cpu.prof -benchmem ./benchmarks/
//
// For memory profiling:
//   go test -bench=BenchmarkFullValidation -memprofile=mem.prof -benchmem ./benchmarks/
//
// Compare results (install benchstat: go install golang.org/x/perf/cmd/benchstat@latest):
//   benchstat baseline.txt optimized.txt

package benchmarks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"

	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi-validator/radix"
)

// --- Test data ---

var (
	adsAPISpec   []byte
	petstoreSpec []byte
	specLoadOnce sync.Once
	adsAPIDoc    libopenapi.Document
	petstoreDoc  libopenapi.Document
	docBuildOnce sync.Once
)

func loadSpecs() {
	specLoadOnce.Do(func() {
		var err error
		adsAPISpec, err = os.ReadFile("../test_specs/ads_api_bulk_actions.yaml")
		if err != nil {
			panic(fmt.Sprintf("failed to read ads_api_bulk_actions.yaml: %v", err))
		}
		petstoreSpec, err = os.ReadFile("../test_specs/petstorev3.json")
		if err != nil {
			panic(fmt.Sprintf("failed to read petstorev3.json: %v", err))
		}
	})
}

func buildDocs() {
	loadSpecs()
	docBuildOnce.Do(func() {
		var err error
		adsAPIDoc, err = libopenapi.NewDocument(adsAPISpec)
		if err != nil {
			panic(fmt.Sprintf("failed to create ads-api document: %v", err))
		}
		petstoreDoc, err = libopenapi.NewDocument(petstoreSpec)
		if err != nil {
			panic(fmt.Sprintf("failed to create petstore document: %v", err))
		}
	})
}

// --- Payload generators ---

// smallBulkActionPayload creates a minimal bulk action request (1 action)
func smallBulkActionPayload() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_campaign_1",
				"entity_data": map[string]interface{}{
					"name":                  "Test Campaign",
					"objective":             "CONVERSIONS",
					"daily_budget_micro":    5000000,
					"start_time":            "2025-01-15T00:00:00Z",
					"funding_instrument_id": "fi_abc123",
					"configured_status":     "PAUSED",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// mediumBulkActionPayload creates a typical bulk action request (4 actions: campaign + ad_group + ad + post)
func mediumBulkActionPayload() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_campaign_1",
				"entity_data": map[string]interface{}{
					"name":                  "Benchmark Campaign",
					"objective":             "CONVERSIONS",
					"daily_budget_micro":    5000000,
					"start_time":            "2025-01-15T00:00:00Z",
					"funding_instrument_id": "fi_abc123",
					"configured_status":     "PAUSED",
				},
			},
			{
				"type":         "AD_GROUP",
				"action":       "CREATE",
				"reference_id": "ref_ad_group_1",
				"entity_data": map[string]interface{}{
					"name":         "Benchmark Ad Group",
					"campaign_id":  "{{ref_campaign_1}}",
					"bid_strategy": "AUTO",
					"goal_type":    "CONVERSIONS",
					"start_time":   "2025-01-15T00:00:00Z",
					"targeting": map[string]interface{}{
						"geos": map[string]interface{}{
							"included": []map[string]interface{}{
								{"country": "US"},
								{"country": "CA"},
							},
						},
						"devices":   []string{"DESKTOP", "MOBILE"},
						"age_range": map[string]interface{}{"min": 18, "max": 54},
						"gender":    "ALL",
						"interests": []map[string]interface{}{
							{"id": "int_1", "name": "Technology"},
							{"id": "int_2", "name": "Gaming"},
						},
					},
				},
			},
			{
				"type":         "POST",
				"action":       "CREATE",
				"reference_id": "ref_post_1",
				"entity_data": map[string]interface{}{
					"headline":  "Check out our new product!",
					"body":      "This is an amazing product that you should definitely check out.",
					"post_type": "IMAGE",
					"content": []map[string]interface{}{
						{
							"type":            "IMAGE",
							"url":             "https://example.com/image.jpg",
							"destination_url": "https://example.com/landing",
							"call_to_action":  "SHOP_NOW",
						},
					},
				},
			},
			{
				"type":         "AD",
				"action":       "CREATE",
				"reference_id": "ref_ad_1",
				"entity_data": map[string]interface{}{
					"name":        "Benchmark Ad",
					"ad_group_id": "{{ref_ad_group_1}}",
					"post_id":     "{{ref_post_1}}",
					"click_url":   "https://example.com/landing?utm_source=reddit",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// largeBulkActionPayload creates a large bulk action request (20 actions)
func largeBulkActionPayload() []byte {
	actions := make([]map[string]interface{}, 0, 20)

	for i := 0; i < 5; i++ {
		actions = append(actions, map[string]interface{}{
			"type":         "CAMPAIGN",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_campaign_%d", i),
			"entity_data": map[string]interface{}{
				"name":                  fmt.Sprintf("Campaign %d", i),
				"objective":             "CONVERSIONS",
				"daily_budget_micro":    5000000 + (i * 1000000),
				"start_time":            "2025-01-15T00:00:00Z",
				"funding_instrument_id": "fi_abc123",
				"configured_status":     "PAUSED",
			},
		})

		actions = append(actions, map[string]interface{}{
			"type":         "AD_GROUP",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_ad_group_%d", i),
			"entity_data": map[string]interface{}{
				"name":         fmt.Sprintf("Ad Group %d", i),
				"campaign_id":  fmt.Sprintf("{{ref_campaign_%d}}", i),
				"bid_strategy": "AUTO",
				"goal_type":    "CONVERSIONS",
				"start_time":   "2025-01-15T00:00:00Z",
				"targeting": map[string]interface{}{
					"geos": map[string]interface{}{
						"included": []map[string]interface{}{
							{"country": "US"},
							{"country": "CA"},
							{"country": "GB"},
						},
					},
					"devices": []string{"DESKTOP", "MOBILE", "TABLET"},
					"os":      []string{"IOS", "ANDROID"},
					"interests": []map[string]interface{}{
						{"id": fmt.Sprintf("int_%d", i), "name": "Technology"},
						{"id": fmt.Sprintf("int_%d", i+10), "name": "Gaming"},
						{"id": fmt.Sprintf("int_%d", i+20), "name": "Sports"},
					},
					"communities": []map[string]interface{}{
						{"id": fmt.Sprintf("com_%d", i), "name": "r/technology"},
						{"id": fmt.Sprintf("com_%d", i+10), "name": "r/gaming"},
					},
					"placements": []string{"FEED", "CONVERSATION"},
					"age_range":  map[string]interface{}{"min": 18, "max": 65},
					"gender":     "ALL",
				},
			},
		})

		actions = append(actions, map[string]interface{}{
			"type":         "POST",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_post_%d", i),
			"entity_data": map[string]interface{}{
				"headline":  fmt.Sprintf("Amazing Product #%d", i),
				"body":      "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
				"post_type": "IMAGE",
				"content": []map[string]interface{}{
					{
						"type":            "IMAGE",
						"url":             fmt.Sprintf("https://example.com/image_%d.jpg", i),
						"destination_url": fmt.Sprintf("https://example.com/landing/%d", i),
						"call_to_action":  "LEARN_MORE",
					},
				},
			},
		})

		actions = append(actions, map[string]interface{}{
			"type":         "AD",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_ad_%d", i),
			"entity_data": map[string]interface{}{
				"name":        fmt.Sprintf("Ad %d", i),
				"ad_group_id": fmt.Sprintf("{{ref_ad_group_%d}}", i),
				"post_id":     fmt.Sprintf("{{ref_post_%d}}", i),
				"click_url":   fmt.Sprintf("https://example.com/landing/%d?utm_source=reddit&utm_medium=cpc", i),
			},
		})
	}

	payload := map[string]interface{}{
		"data": actions,
	}
	b, _ := json.Marshal(payload)
	return b
}

// simplePetstorePayload creates a valid Pet JSON payload
func simplePetstorePayload() []byte {
	payload := map[string]interface{}{
		"id":   10,
		"name": "doggie",
		"category": map[string]interface{}{
			"id":   1,
			"name": "Dogs",
		},
		"photoUrls": []string{"https://example.com/photo.jpg"},
		"tags": []map[string]interface{}{
			{"id": 1, "name": "friendly"},
		},
		"status": "available",
	}
	b, _ := json.Marshal(payload)
	return b
}

// === SECTION 1: Validator Initialization Benchmarks ===
// These measure the cost of building a validator (parsing, schema warming, radix tree)

func BenchmarkValidatorInit_AdsAPI(b *testing.B) {
	loadSpecs()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		doc, err := libopenapi.NewDocument(adsAPISpec)
		if err != nil {
			b.Fatal(err)
		}
		v, errs := validator.NewValidator(doc)
		if errs != nil {
			b.Fatal(errs)
		}
		_ = v
	}
}

func BenchmarkValidatorInit_Petstore(b *testing.B) {
	loadSpecs()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		doc, err := libopenapi.NewDocument(petstoreSpec)
		if err != nil {
			b.Fatal(err)
		}
		v, errs := validator.NewValidator(doc)
		if errs != nil {
			b.Fatal(errs)
		}
		_ = v
	}
}

func BenchmarkValidatorInit_AdsAPI_WithoutSchemaCache(b *testing.B) {
	loadSpecs()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		doc, err := libopenapi.NewDocument(adsAPISpec)
		if err != nil {
			b.Fatal(err)
		}
		v, errs := validator.NewValidator(doc, config.WithSchemaCache(nil))
		if errs != nil {
			b.Fatal(errs)
		}
		_ = v
	}
}

func BenchmarkValidatorInit_AdsAPI_WithoutRadixTree(b *testing.B) {
	loadSpecs()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		doc, err := libopenapi.NewDocument(adsAPISpec)
		if err != nil {
			b.Fatal(err)
		}
		v, errs := validator.NewValidator(doc, config.WithPathTree(nil))
		if errs != nil {
			b.Fatal(errs)
		}
		_ = v
	}
}

// BenchmarkValidatorInit_AdsAPI_MemoryFootprint measures the memory cost of keeping a validator alive.
func BenchmarkValidatorInit_AdsAPI_MemoryFootprint(b *testing.B) {
	loadSpecs()

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	doc, err := libopenapi.NewDocument(adsAPISpec)
	if err != nil {
		b.Fatal(err)
	}
	v, errs := validator.NewValidator(doc)
	if errs != nil {
		b.Fatal(errs)
	}

	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	heapDelta := memAfter.HeapAlloc - memBefore.HeapAlloc
	b.ReportMetric(float64(heapDelta), "heap-bytes")
	b.ReportMetric(float64(memAfter.HeapObjects-memBefore.HeapObjects), "heap-objects")

	// Keep validator alive
	_ = v
}

// === SECTION 2: Path Matching Benchmarks ===
// These isolate path lookup performance: radix tree vs regex fallback

func BenchmarkPathMatch_RadixTree_LiteralPath(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions()
	opts.PathTree = radix.BuildPathTree(&doc.Model)

	req, _ := http.NewRequest(http.MethodGet, "/api/v3/ad_accounts", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		paths.FindPath(req, &doc.Model, opts)
	}
}

func BenchmarkPathMatch_RadixTree_SingleParam(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions()
	opts.PathTree = radix.BuildPathTree(&doc.Model)

	req, _ := http.NewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_12345", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		paths.FindPath(req, &doc.Model, opts)
	}
}

func BenchmarkPathMatch_RadixTree_DeepParam(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions()
	opts.PathTree = radix.BuildPathTree(&doc.Model)

	req, _ := http.NewRequest(http.MethodPost, "/api/v3/ad_accounts/acc_12345/bulk_actions", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		paths.FindPath(req, &doc.Model, opts)
	}
}

func BenchmarkPathMatch_RegexFallback_LiteralPath(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions(config.WithPathTree(nil))

	req, _ := http.NewRequest(http.MethodGet, "/api/v3/ad_accounts", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		paths.FindPath(req, &doc.Model, opts)
	}
}

func BenchmarkPathMatch_RegexFallback_SingleParam(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions(config.WithPathTree(nil))

	req, _ := http.NewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_12345", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		paths.FindPath(req, &doc.Model, opts)
	}
}

func BenchmarkPathMatch_RegexFallback_DeepParam(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions(config.WithPathTree(nil))

	req, _ := http.NewRequest(http.MethodPost, "/api/v3/ad_accounts/acc_12345/bulk_actions", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		paths.FindPath(req, &doc.Model, opts)
	}
}

func BenchmarkPathMatch_RegexFallback_WithCache(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	regexCache := &sync.Map{}
	opts := config.NewValidationOptions(config.WithPathTree(nil), config.WithRegexCache(regexCache))

	req, _ := http.NewRequest(http.MethodPost, "/api/v3/ad_accounts/acc_12345/bulk_actions", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		paths.FindPath(req, &doc.Model, opts)
	}
}

// BenchmarkPathMatch_AllEndpoints tests path matching across ALL endpoints to simulate real traffic
func BenchmarkPathMatch_AllEndpoints_RadixTree(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions()
	opts.PathTree = radix.BuildPathTree(&doc.Model)

	requests := []*http.Request{
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/campaigns"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/campaigns/camp_456"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ad_groups"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ad_groups/ag_789"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ads"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ads/ad_012"),
		mustNewRequest(http.MethodPost, "/api/v3/ad_accounts/acc_123/bulk_actions"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/bulk_actions/job_345"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/posts"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/posts/post_678"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/pixels"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/funding_instruments"),
		mustNewRequest(http.MethodPost, "/api/v3/ad_accounts/acc_123/reporting"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/custom_audiences"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/saved_audiences"),
		mustNewRequest(http.MethodGet, "/api/v3/businesses"),
		mustNewRequest(http.MethodGet, "/api/v3/me"),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := requests[i%len(requests)]
		paths.FindPath(req, &doc.Model, opts)
	}
}

func BenchmarkPathMatch_AllEndpoints_RegexFallback(b *testing.B) {
	buildDocs()
	doc, _ := adsAPIDoc.BuildV3Model()
	opts := config.NewValidationOptions(config.WithPathTree(nil))

	requests := []*http.Request{
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/campaigns"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/campaigns/camp_456"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ad_groups"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ad_groups/ag_789"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ads"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/ads/ad_012"),
		mustNewRequest(http.MethodPost, "/api/v3/ad_accounts/acc_123/bulk_actions"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/bulk_actions/job_345"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/posts"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/posts/post_678"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/pixels"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/funding_instruments"),
		mustNewRequest(http.MethodPost, "/api/v3/ad_accounts/acc_123/reporting"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/custom_audiences"),
		mustNewRequest(http.MethodGet, "/api/v3/ad_accounts/acc_123/saved_audiences"),
		mustNewRequest(http.MethodGet, "/api/v3/businesses"),
		mustNewRequest(http.MethodGet, "/api/v3/me"),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := requests[i%len(requests)]
		paths.FindPath(req, &doc.Model, opts)
	}
}

// === SECTION 3: Request Body Validation Benchmarks ===
// These measure the cost of schema validation with different payload sizes

func BenchmarkRequestValidation_BulkActions_Small(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := smallBulkActionPayload()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkRequestValidation_BulkActions_Medium(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkRequestValidation_BulkActions_Large(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := largeBulkActionPayload()
	b.ReportMetric(float64(len(payload)), "payload-bytes")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkRequestValidation_Petstore_AddPet(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(petstoreDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := simplePetstorePayload()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/pet",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

// === SECTION 4: Sync vs Async Validation ===
// Measures the goroutine overhead of async validation

func BenchmarkRequestValidation_BulkActions_Sync(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequestSync(req)
	}
}

func BenchmarkRequestValidation_BulkActions_Async(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

// === SECTION 5: GET Request Validation (path + params only, no body) ===

func BenchmarkRequestValidation_GET_Simple(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/campaigns/camp_67890", nil)
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkRequestValidation_GET_WithQueryParams(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/campaigns?page_size=50&page_token=abc123", nil)
		v.ValidateHttpRequest(req)
	}
}

// === SECTION 6: Concurrent Validation ===
// Simulates production load with concurrent requests

func BenchmarkConcurrentValidation_BulkActions(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest(http.MethodPost,
				"/api/v3/ad_accounts/acc_12345/bulk_actions",
				bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			v.ValidateHttpRequest(req)
		}
	})
}

func BenchmarkConcurrentValidation_MixedEndpoints(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()

	type testCase struct {
		method  string
		url     string
		payload []byte
	}

	cases := []testCase{
		{http.MethodGet, "/api/v3/ad_accounts/acc_123", nil},
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/campaigns", nil},
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/campaigns/camp_456", nil},
		{http.MethodPost, "/api/v3/ad_accounts/acc_123/bulk_actions", payload},
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/ads/ad_789", nil},
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/posts/post_012", nil},
		{http.MethodGet, "/api/v3/businesses", nil},
		{http.MethodGet, "/api/v3/me", nil},
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			tc := cases[idx%len(cases)]
			var req *http.Request
			if tc.payload != nil {
				req, _ = http.NewRequest(tc.method, tc.url, bytes.NewReader(tc.payload))
			} else {
				req, _ = http.NewRequest(tc.method, tc.url, nil)
			}
			if tc.payload != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			v.ValidateHttpRequest(req)
			idx++
		}
	})
}

// === SECTION 7: Per-request memory allocation analysis ===
// These benchmarks are designed for detailed memory profiling

func BenchmarkMemory_SingleValidation_BulkActions(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()
	b.ReportMetric(float64(len(payload)), "payload-bytes")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		valid, validationErrs := v.ValidateHttpRequest(req)
		_ = valid
		_ = validationErrs
	}
}

func BenchmarkMemory_SingleValidation_GET(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/campaigns/camp_67890", nil)
		valid, validationErrs := v.ValidateHttpRequest(req)
		_ = valid
		_ = validationErrs
	}
}

// === SECTION 8: Schema Cache Impact ===

func BenchmarkRequestValidation_WithSchemaCache(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkRequestValidation_WithoutSchemaCache(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc, config.WithSchemaCache(nil))
	if errs != nil {
		b.Fatal(errs)
	}

	payload := mediumBulkActionPayload()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

// === SECTION 9: Endpoint count scaling ===
// Tests how the number of endpoints affects validation performance

func BenchmarkPathMatch_ScaleEndpoints(b *testing.B) {
	for _, numEndpoints := range []int{5, 10, 25, 50, 100} {
		b.Run(fmt.Sprintf("RadixTree_%d_endpoints", numEndpoints), func(b *testing.B) {
			spec := generateScalingSpec(numEndpoints)
			doc, err := libopenapi.NewDocument(spec)
			if err != nil {
				b.Fatal(err)
			}
			model, _ := doc.BuildV3Model()
			opts := config.NewValidationOptions()
			opts.PathTree = radix.BuildPathTree(&model.Model)

			// Target a path in the middle
			target := fmt.Sprintf("/api/v3/resource_%d/item_abc", numEndpoints/2)
			req, _ := http.NewRequest(http.MethodGet, target, nil)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				paths.FindPath(req, &model.Model, opts)
			}
		})

		b.Run(fmt.Sprintf("RegexFallback_%d_endpoints", numEndpoints), func(b *testing.B) {
			spec := generateScalingSpec(numEndpoints)
			doc, err := libopenapi.NewDocument(spec)
			if err != nil {
				b.Fatal(err)
			}
			model, _ := doc.BuildV3Model()
			opts := config.NewValidationOptions(config.WithPathTree(nil))

			target := fmt.Sprintf("/api/v3/resource_%d/item_abc", numEndpoints/2)
			req, _ := http.NewRequest(http.MethodGet, target, nil)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				paths.FindPath(req, &model.Model, opts)
			}
		})
	}
}

// === SECTION 10: Response Validation ===
// These measure the cost of validating response bodies against the spec.

func validBulkActionsResp() []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"id":     "job_abc123",
			"status": "SUCCESS",
			"input": []map[string]interface{}{
				{
					"type":         "CAMPAIGN",
					"action":       "CREATE",
					"reference_id": "ref_campaign_1",
					"entity_data": map[string]interface{}{
						"name":                  "Campaign 1",
						"objective":             "CONVERSIONS",
						"daily_budget_micro":    5000000,
						"start_time":            "2025-01-15T00:00:00Z",
						"funding_instrument_id": "fi_abc123",
						"configured_status":     "PAUSED",
					},
				},
			},
			"results": []map[string]interface{}{
				{
					"reference_id": "ref_campaign_1",
					"type":         "CAMPAIGN",
					"status":       "SUCCESS",
					"id":           "camp_xyz",
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func validCampaignResp() []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"id":        "camp_xyz",
			"name":      "Test Campaign",
			"status":    "ACTIVE",
			"objective": "CONVERSIONS",
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func makeTestResponse(statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func BenchmarkResponseValidation_BulkActions(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	reqPayload := smallBulkActionPayload()
	respBody := validBulkActionsResp()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(reqPayload))
		req.Header.Set("Content-Type", "application/json")
		resp := makeTestResponse(201, respBody)
		v.ValidateHttpResponse(req, resp)
	}
}

func BenchmarkResponseValidation_GET_Campaign(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	respBody := validCampaignResp()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/campaigns/camp_67890", nil)
		resp := makeTestResponse(200, respBody)
		v.ValidateHttpResponse(req, resp)
	}
}

func BenchmarkRequestResponseValidation_BulkActions(b *testing.B) {
	buildDocs()
	v, errs := validator.NewValidator(adsAPIDoc)
	if errs != nil {
		b.Fatal(errs)
	}

	reqPayload := mediumBulkActionPayload()
	respBody := validBulkActionsResp()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(reqPayload))
		req.Header.Set("Content-Type", "application/json")
		resp := makeTestResponse(201, respBody)
		v.ValidateHttpRequestResponse(req, resp)
	}
}

// === Helpers ===

func mustNewRequest(method, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	return req
}

// generateScalingSpec generates a minimal OpenAPI spec with N endpoint pairs (list + get-by-id)
func generateScalingSpec(numEndpoints int) []byte {
	var pathsYAML string
	for i := 0; i < numEndpoints; i++ {
		pathsYAML += fmt.Sprintf(`  /resource_%d:
    get:
      operationId: listResource%d
      responses:
        "200":
          description: OK
  /resource_%d/{item_id}:
    get:
      operationId: getResource%d
      parameters:
        - name: item_id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
`, i, i, i, i)
	}

	spec := fmt.Sprintf(`openapi: "3.0.2"
info:
  title: Scaling Benchmark
  version: "1.0.0"
servers:
  - url: /api/v3
paths:
%s`, pathsYAML)

	return []byte(spec)
}
