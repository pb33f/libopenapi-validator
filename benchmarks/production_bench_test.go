// Benchmarks using the production Reddit Ads API complete.yaml spec.
//
// This uses the real production OpenAPI spec for accurate performance measurements.
// Includes both valid payloads (should pass validation) and invalid payloads (should fail).
//
// Run benchmarks:
//   go test -bench=BenchmarkProd -benchmem -count=3 -timeout=10m ./benchmarks/
//
// Run validation correctness tests:
//   go test -run=TestProdPayload -v ./benchmarks/

package benchmarks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"

	validator "github.com/pb33f/libopenapi-validator"
)

var (
	prodSpec     []byte
	prodSpecOnce sync.Once

	prodDoc     libopenapi.Document
	prodDocOnce sync.Once
)

func loadProdSpec(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping production spec test in short mode")
	}
	prodSpecOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("cannot get home dir: %v", err)
			return
		}
		specPath := filepath.Join(home, "src", "ads-api", "open_api_spec", "v3", "complete.yaml")
		prodSpec, err = os.ReadFile(specPath)
		if err != nil {
			t.Skipf("production spec not found at %s: %v", specPath, err)
			return
		}
	})
	if prodSpec == nil {
		t.Skip("production spec not available")
	}
}

func buildProdDoc(t testing.TB) {
	t.Helper()
	loadProdSpec(t)
	prodDocOnce.Do(func() {
		var err error
		prodDoc, err = libopenapi.NewDocument(prodSpec)
		if err != nil {
			panic(fmt.Sprintf("failed to parse production spec: %v", err))
		}
	})
}

func newProdValidator(t testing.TB) validator.Validator {
	t.Helper()
	buildProdDoc(t)
	v, errs := validator.NewValidator(prodDoc)
	if errs != nil {
		t.Fatalf("failed to create validator: %v", errs)
	}
	return v
}

// ---------------------------------------------------------------------------
// Payloads: VALID
// ---------------------------------------------------------------------------

// validCampaignAction - a single Campaign bulk action (minimal required fields)
func validCampaignAction() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "CAMPAIGN",
				"action": "CREATE",
				"entity_data": map[string]interface{}{
					"name":      "Benchmark Campaign",
					"objective": "CONVERSIONS",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// validMixedBulkActions - 3 different entity types typical for a real BulkActions call
func validMixedBulkActions() []byte {
	payload := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_campaign_1",
				"entity_data": map[string]interface{}{
					"name":      "Perf Test Campaign",
					"objective": "CONVERSIONS",
				},
			},
			map[string]interface{}{
				"type":         "AD_GROUP",
				"action":       "CREATE",
				"reference_id": "ref_ag_1",
				"entity_data": map[string]interface{}{
					"name":         "Perf Test Ad Group",
					"campaign_id":  "{{ref_campaign_1}}",
					"bid_strategy": "MAXIMIZE_VOLUME",
					"bid_type":     "CPC",
					"start_time":   "2025-06-01T00:00:00Z",
				},
			},
			map[string]interface{}{
				"type":         "POST",
				"action":       "CREATE",
				"reference_id": "ref_post_1",
				"entity_data": map[string]interface{}{
					"creative": map[string]interface{}{
						"type":     "IMAGE",
						"headline": "Check out our product!",
						"destination": map[string]interface{}{
							"url":  "https://example.com/landing",
							"type": "URL",
						},
						"image": map[string]interface{}{
							"media": map[string]interface{}{
								"url":  "https://example.com/image.jpg",
								"type": "URL",
							},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// validLargeBulkActions - 15 items across entity types (realistic large batch)
func validLargeBulkActions() []byte {
	actions := make([]interface{}, 0, 15)

	for i := 0; i < 5; i++ {
		actions = append(actions, map[string]interface{}{
			"type":         "CAMPAIGN",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_camp_%d", i),
			"entity_data": map[string]interface{}{
				"name":      fmt.Sprintf("Campaign %d", i),
				"objective": "CONVERSIONS",
			},
		})
		actions = append(actions, map[string]interface{}{
			"type":         "AD_GROUP",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_ag_%d", i),
			"entity_data": map[string]interface{}{
				"name":         fmt.Sprintf("Ad Group %d", i),
				"campaign_id":  fmt.Sprintf("{{ref_camp_%d}}", i),
				"bid_strategy": "MAXIMIZE_VOLUME",
				"bid_type":     "CPC",
				"start_time":   "2025-06-01T00:00:00Z",
			},
		})
		actions = append(actions, map[string]interface{}{
			"type":         "POST",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_post_%d", i),
			"entity_data": map[string]interface{}{
				"creative": map[string]interface{}{
					"type":     "IMAGE",
					"headline": fmt.Sprintf("Product %d!", i),
					"destination": map[string]interface{}{
						"url":  fmt.Sprintf("https://example.com/landing/%d", i),
						"type": "URL",
					},
					"image": map[string]interface{}{
						"media": map[string]interface{}{
							"url":  fmt.Sprintf("https://example.com/image_%d.jpg", i),
							"type": "URL",
						},
					},
				},
			},
		})
	}

	payload := map[string]interface{}{"data": actions}
	b, _ := json.Marshal(payload)
	return b
}

// ---------------------------------------------------------------------------
// Payloads: INVALID (should produce validation errors)
// ---------------------------------------------------------------------------

// invalidMissingEntityData - missing required field "entity_data"
func invalidMissingEntityData() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "CAMPAIGN",
				"action": "CREATE",
				// entity_data is missing — required by schema
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// invalidWrongType - type field has an invalid enum value
func invalidWrongType() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "INVALID_TYPE",
				"action": "CREATE",
				"entity_data": map[string]interface{}{
					"name":      "Bad Campaign",
					"objective": "CONVERSIONS",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// invalidExtraTopLevelField - additionalProperties is false, so unknown fields are invalid
func invalidExtraTopLevelField() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "CAMPAIGN",
				"action": "CREATE",
				"entity_data": map[string]interface{}{
					"name":      "Campaign",
					"objective": "CONVERSIONS",
				},
				"this_field_does_not_exist": "boom",
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// invalidEmptyData - data array is empty (should fail minItems if defined, or at least be a degenerate case)
func invalidEmptyData() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{},
	}
	b, _ := json.Marshal(payload)
	return b
}

// invalidWrongContentType generates a valid payload but uses the wrong content type
// (this is tested at the request level, not at the payload level)

// ---------------------------------------------------------------------------
// Validation correctness tests (not benchmarks)
// ---------------------------------------------------------------------------

func TestProdPayload_ValidCampaignAction(t *testing.T) {
	v := newProdValidator(t)
	payload := validCampaignAction()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	valid, errs := v.ValidateHttpRequest(req)
	if !valid {
		t.Logf("Validation errors for validCampaignAction:")
		for _, e := range errs {
			t.Logf("  - %s", e.Message)
		}
	}
	t.Logf("validCampaignAction: valid=%v, errors=%d, payload=%d bytes", valid, len(errs), len(payload))
}

func TestProdPayload_ValidMixedBulkActions(t *testing.T) {
	v := newProdValidator(t)
	payload := validMixedBulkActions()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	valid, errs := v.ValidateHttpRequest(req)
	if !valid {
		t.Logf("Validation errors for validMixedBulkActions:")
		for _, e := range errs {
			t.Logf("  - %s", e.Message)
		}
	}
	t.Logf("validMixedBulkActions: valid=%v, errors=%d, payload=%d bytes", valid, len(errs), len(payload))
}

func TestProdPayload_ValidLargeBulkActions(t *testing.T) {
	v := newProdValidator(t)
	payload := validLargeBulkActions()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	valid, errs := v.ValidateHttpRequest(req)
	if !valid {
		t.Logf("Validation errors for validLargeBulkActions:")
		for _, e := range errs {
			t.Logf("  - %s", e.Message)
		}
	}
	t.Logf("validLargeBulkActions: valid=%v, errors=%d, payload=%d bytes", valid, len(errs), len(payload))
}

func TestProdPayload_InvalidMissingEntityData(t *testing.T) {
	v := newProdValidator(t)
	payload := invalidMissingEntityData()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("invalidMissingEntityData: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
	if valid {
		t.Error("expected validation to fail for missing entity_data, but it passed")
	}
}

func TestProdPayload_InvalidWrongType(t *testing.T) {
	v := newProdValidator(t)
	payload := invalidWrongType()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("invalidWrongType: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
	if valid {
		t.Error("expected validation to fail for wrong type enum, but it passed")
	}
}

func TestProdPayload_InvalidExtraField(t *testing.T) {
	v := newProdValidator(t)
	payload := invalidExtraTopLevelField()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("invalidExtraField: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
	if valid {
		t.Error("expected validation to fail for extra field (additionalProperties:false), but it passed")
	}
}

func TestProdPayload_InvalidContentType(t *testing.T) {
	v := newProdValidator(t)
	payload := validCampaignAction()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "text/plain") // wrong content type
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("invalidContentType: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
	if valid {
		t.Error("expected validation to fail for wrong content type, but it passed")
	}
}

func TestProdPayload_ValidGET_ListCampaigns(t *testing.T) {
	v := newProdValidator(t)
	req, _ := http.NewRequest(http.MethodGet,
		"/api/v3/ad_accounts/acc_12345/campaigns?page.size=25", nil)
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("validGET_ListCampaigns: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
}

func TestProdPayload_ValidGET_ListAds(t *testing.T) {
	v := newProdValidator(t)
	req, _ := http.NewRequest(http.MethodGet,
		"/api/v3/ad_accounts/acc_12345/ads?page.size=50", nil)
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("validGET_ListAds: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
}

func TestProdPayload_ValidGET_ListAdGroups(t *testing.T) {
	v := newProdValidator(t)
	req, _ := http.NewRequest(http.MethodGet,
		"/api/v3/ad_accounts/acc_12345/ad_groups?campaign_id=camp_123&page.size=25", nil)
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("validGET_ListAdGroups: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
}

func TestProdPayload_InvalidGET_UnknownPath(t *testing.T) {
	v := newProdValidator(t)
	req, _ := http.NewRequest(http.MethodGet,
		"/api/v3/this/path/does/not/exist", nil)
	valid, errs := v.ValidateHttpRequest(req)
	t.Logf("invalidGET_UnknownPath: valid=%v, errors=%d", valid, len(errs))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
	}
	if valid {
		t.Error("expected validation to fail for unknown path, but it passed")
	}
}

// ---------------------------------------------------------------------------
// Benchmarks: Initialization
// ---------------------------------------------------------------------------

func BenchmarkProd_Init(b *testing.B) {
	loadProdSpec(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		doc, err := libopenapi.NewDocument(prodSpec)
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

func BenchmarkProd_Init_MemoryFootprint(b *testing.B) {
	loadProdSpec(b)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	doc, err := libopenapi.NewDocument(prodSpec)
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
	_ = v
}

// ---------------------------------------------------------------------------
// Benchmarks: POST Bulk Actions — valid payloads
// ---------------------------------------------------------------------------

func BenchmarkProd_BulkActions_SingleCampaign(b *testing.B) {
	v := newProdValidator(b)
	payload := validCampaignAction()
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

func BenchmarkProd_BulkActions_Mixed3(b *testing.B) {
	v := newProdValidator(b)
	payload := validMixedBulkActions()
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

func BenchmarkProd_BulkActions_Large15(b *testing.B) {
	v := newProdValidator(b)
	payload := validLargeBulkActions()
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

// ---------------------------------------------------------------------------
// Benchmarks: POST Bulk Actions — invalid payloads (measures error path cost)
// ---------------------------------------------------------------------------

func BenchmarkProd_BulkActions_Invalid_MissingField(b *testing.B) {
	v := newProdValidator(b)
	payload := invalidMissingEntityData()
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

func BenchmarkProd_BulkActions_Invalid_WrongType(b *testing.B) {
	v := newProdValidator(b)
	payload := invalidWrongType()
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

func BenchmarkProd_BulkActions_Invalid_ExtraField(b *testing.B) {
	v := newProdValidator(b)
	payload := invalidExtraTopLevelField()
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

func BenchmarkProd_BulkActions_Invalid_ContentType(b *testing.B) {
	v := newProdValidator(b)
	payload := validCampaignAction()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "text/plain")
		v.ValidateHttpRequest(req)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks: GET endpoints (path params + query params, no body)
// ---------------------------------------------------------------------------

func BenchmarkProd_GET_ListCampaigns(b *testing.B) {
	v := newProdValidator(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/campaigns?page.size=25", nil)
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkProd_GET_ListAdGroups(b *testing.B) {
	v := newProdValidator(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/ad_groups?campaign_id=camp_123&page.size=25", nil)
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkProd_GET_ListAds(b *testing.B) {
	v := newProdValidator(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/ads?page.size=50", nil)
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkProd_GET_Me(b *testing.B) {
	v := newProdValidator(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/me", nil)
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkProd_GET_GetBulkActionsJob(b *testing.B) {
	v := newProdValidator(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/bulk_actions/job_67890", nil)
		v.ValidateHttpRequest(req)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks: Concurrent validation (simulates production load)
// ---------------------------------------------------------------------------

func BenchmarkProd_Concurrent_BulkActions(b *testing.B) {
	v := newProdValidator(b)
	payload := validMixedBulkActions()
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

func BenchmarkProd_Concurrent_MixedEndpoints(b *testing.B) {
	v := newProdValidator(b)
	payload := validMixedBulkActions()

	type testCase struct {
		method  string
		url     string
		payload []byte
	}

	cases := []testCase{
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/campaigns?page.size=25", nil},
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/ad_groups?page.size=25", nil},
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/ads?page.size=50", nil},
		{http.MethodPost, "/api/v3/ad_accounts/acc_123/bulk_actions", payload},
		{http.MethodGet, "/api/v3/me", nil},
		{http.MethodGet, "/api/v3/ad_accounts/acc_123/bulk_actions/job_456", nil},
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
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(tc.method, tc.url, nil)
			}
			v.ValidateHttpRequest(req)
			idx++
		}
	})
}

// ---------------------------------------------------------------------------
// Benchmarks: Sync vs Async
// ---------------------------------------------------------------------------

func BenchmarkProd_BulkActions_Sync(b *testing.B) {
	v := newProdValidator(b)
	payload := validMixedBulkActions()
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

func BenchmarkProd_BulkActions_Async(b *testing.B) {
	v := newProdValidator(b)
	payload := validMixedBulkActions()
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

// ---------------------------------------------------------------------------
// Response payloads
// ---------------------------------------------------------------------------

// validBulkActionsResponse - a realistic 201 response from POST bulk_actions
func validBulkActionsResponse() []byte {
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
						"name":      "Campaign 1",
						"objective": "CONVERSIONS",
					},
				},
				{
					"type":         "AD_GROUP",
					"action":       "CREATE",
					"reference_id": "ref_ag_1",
					"entity_data": map[string]interface{}{
						"name":         "Ad Group 1",
						"campaign_id":  "camp_xyz",
						"bid_strategy": "MAXIMIZE_VOLUME",
						"bid_type":     "CPC",
						"start_time":   "2025-06-01T00:00:00Z",
					},
				},
			},
			"results": []map[string]interface{}{
				{
					"id":           "result_1",
					"reference_id": "ref_campaign_1",
					"type":         "CAMPAIGN",
					"status":       "SUCCESS",
					"errors":       []interface{}{},
					"suggestions":  []interface{}{},
				},
				{
					"id":           "result_2",
					"reference_id": "ref_ag_1",
					"type":         "AD_GROUP",
					"status":       "SUCCESS",
					"errors":       []interface{}{},
					"suggestions":  []interface{}{},
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// validListCampaignsResponse - a realistic 200 response from GET campaigns
func validListCampaignsResponse() []byte {
	campaigns := make([]map[string]interface{}, 0, 5)
	for i := 0; i < 5; i++ {
		campaigns = append(campaigns, map[string]interface{}{
			"id":                fmt.Sprintf("t2_camp_%d", i),
			"ad_account_id":     "t2_acc_12345",
			"name":              fmt.Sprintf("Campaign %d", i),
			"objective":         "CONVERSIONS",
			"configured_status": "ACTIVE",
			"effective_status":  "ACTIVE",
			"created_at":        "2025-01-15T00:00:00Z",
		})
	}
	resp := map[string]interface{}{
		"data":       campaigns,
		"pagination": map[string]interface{}{},
	}
	b, _ := json.Marshal(resp)
	return b
}

// invalidBulkActionsResponse_ExtraField - response with unknown field
func invalidBulkActionsResponse_ExtraField() []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"id":     "job_abc123",
			"status": "COMPLETED",
			"input":  []map[string]interface{}{},
			"results": []map[string]interface{}{
				{
					"reference_id":    "ref_1",
					"type":            "CAMPAIGN",
					"action":          "CREATE",
					"status":          "SUCCESS",
					"entity_id":       "camp_xyz",
					"unknown_garbage": "this should fail",
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func makeResponse(statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

// ---------------------------------------------------------------------------
// Response validation correctness tests
// ---------------------------------------------------------------------------

func TestProdPayload_ValidBulkActionsResponse(t *testing.T) {
	v := newProdValidator(t)
	reqPayload := validMixedBulkActions()
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(reqPayload))
	req.Header.Set("Content-Type", "application/json")

	respBody := validBulkActionsResponse()
	resp := makeResponse(201, respBody)

	valid, errs := v.ValidateHttpResponse(req, resp)
	t.Logf("ValidBulkActionsResponse: valid=%v, errors=%d, body=%d bytes", valid, len(errs), len(respBody))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
		if e.Reason != "" {
			t.Logf("    reason: %s", e.Reason)
		}
		for j, sve := range e.SchemaValidationErrors {
			t.Logf("    schema[%d] reason: %s", j+1, sve.Reason)
			if sve.Location != "" {
				t.Logf("    schema[%d] location: %s", j+1, sve.Location)
			}
			if sve.FieldPath != "" {
				t.Logf("    schema[%d] fieldPath: %s", j+1, sve.FieldPath)
			}
		}
	}
}

func TestProdPayload_ValidListCampaignsResponse(t *testing.T) {
	v := newProdValidator(t)
	req, _ := http.NewRequest(http.MethodGet,
		"/api/v3/ad_accounts/acc_12345/campaigns?page.size=25", nil)

	respBody := validListCampaignsResponse()
	resp := makeResponse(200, respBody)

	valid, errs := v.ValidateHttpResponse(req, resp)
	t.Logf("ValidListCampaignsResponse: valid=%v, errors=%d, body=%d bytes", valid, len(errs), len(respBody))
	for _, e := range errs {
		t.Logf("  - %s", e.Message)
		if e.Reason != "" {
			t.Logf("    reason: %s", e.Reason)
		}
		for j, sve := range e.SchemaValidationErrors {
			t.Logf("    schema[%d] reason: %s", j+1, sve.Reason)
			if sve.Location != "" {
				t.Logf("    schema[%d] location: %s", j+1, sve.Location)
			}
			if sve.FieldPath != "" {
				t.Logf("    schema[%d] fieldPath: %s", j+1, sve.FieldPath)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmarks: Response Validation
// ---------------------------------------------------------------------------

func BenchmarkProd_ResponseValidation_BulkActions(b *testing.B) {
	v := newProdValidator(b)
	reqPayload := validMixedBulkActions()
	respBody := validBulkActionsResponse()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(reqPayload))
		req.Header.Set("Content-Type", "application/json")
		resp := makeResponse(201, respBody)
		v.ValidateHttpResponse(req, resp)
	}
}

func BenchmarkProd_ResponseValidation_ListCampaigns(b *testing.B) {
	v := newProdValidator(b)
	respBody := validListCampaignsResponse()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v3/ad_accounts/acc_12345/campaigns?page.size=25", nil)
		resp := makeResponse(200, respBody)
		v.ValidateHttpResponse(req, resp)
	}
}

func BenchmarkProd_ResponseValidation_Invalid_ExtraField(b *testing.B) {
	v := newProdValidator(b)
	reqPayload := validCampaignAction()
	respBody := invalidBulkActionsResponse_ExtraField()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(reqPayload))
		req.Header.Set("Content-Type", "application/json")
		resp := makeResponse(201, respBody)
		v.ValidateHttpResponse(req, resp)
	}
}

func BenchmarkProd_RequestResponseValidation_BulkActions(b *testing.B) {
	v := newProdValidator(b)
	reqPayload := validMixedBulkActions()
	respBody := validBulkActionsResponse()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_12345/bulk_actions",
			bytes.NewReader(reqPayload))
		req.Header.Set("Content-Type", "application/json")
		resp := makeResponse(201, respBody)
		v.ValidateHttpRequestResponse(req, resp)
	}
}
