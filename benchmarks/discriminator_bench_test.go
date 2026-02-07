// Benchmarks comparing oneOf+discriminator vs if/then for discriminated unions.
//
// Run with:
//   go test -bench=BenchmarkDiscriminator -benchmem -count=5 -timeout=10m ./benchmarks/
//
// The oneOf approach validates against ALL schemas to find the match.
// The if/then approach only validates against the schema where the if condition passes.

package benchmarks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"

	validator "github.com/pb33f/libopenapi-validator"
)

var (
	oneOfSpec  []byte
	ifThenSpec []byte
	discOnce   sync.Once

	oneOfDoc  libopenapi.Document
	ifThenDoc libopenapi.Document
	discDocs  sync.Once
)

func loadDiscSpecs() {
	discOnce.Do(func() {
		var err error
		oneOfSpec, err = os.ReadFile("../test_specs/discriminator_oneof.yaml")
		if err != nil {
			panic(fmt.Sprintf("failed to read discriminator_oneof.yaml: %v", err))
		}
		ifThenSpec, err = os.ReadFile("../test_specs/discriminator_ifthen.yaml")
		if err != nil {
			panic(fmt.Sprintf("failed to read discriminator_ifthen.yaml: %v", err))
		}
	})
}

func buildDiscDocs() {
	loadDiscSpecs()
	discDocs.Do(func() {
		var err error
		oneOfDoc, err = libopenapi.NewDocument(oneOfSpec)
		if err != nil {
			panic(fmt.Sprintf("failed to create oneOf document: %v", err))
		}
		ifThenDoc, err = libopenapi.NewDocument(ifThenSpec)
		if err != nil {
			panic(fmt.Sprintf("failed to create if/then document: %v", err))
		}
	})
}

// --- Payloads ---

// Single action: validates one item against the discriminated union
func singleCampaignAction() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_1",
				"entity_data": map[string]interface{}{
					"entity_type":           "CAMPAIGN",
					"name":                  "Test Campaign",
					"objective":             "CONVERSIONS",
					"daily_budget_micro":    5000000,
					"start_time":            "2025-01-15T00:00:00Z",
					"funding_instrument_id": "fi_123",
					"configured_status":     "PAUSED",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// Single POST action (tests the 4th branch - should be slower with oneOf since
// it has to fail on CAMPAIGN, AD_GROUP, AD before finding POST)
func singlePostAction() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "POST",
				"action":       "CREATE",
				"reference_id": "ref_1",
				"entity_data": map[string]interface{}{
					"entity_type": "POST",
					"headline":    "Check out our product!",
					"post_type":   "IMAGE",
					"content": []map[string]interface{}{
						{
							"content_type":    "IMAGE",
							"url":             "https://example.com/image.jpg",
							"destination_url": "https://example.com/landing",
							"call_to_action":  "SHOP_NOW",
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// Mixed payload: 4 different entity types (typical BulkActions request)
func mixedBulkActions() []byte {
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_campaign",
				"entity_data": map[string]interface{}{
					"entity_type":           "CAMPAIGN",
					"name":                  "Campaign 1",
					"objective":             "CONVERSIONS",
					"daily_budget_micro":    5000000,
					"start_time":            "2025-01-15T00:00:00Z",
					"funding_instrument_id": "fi_123",
				},
			},
			{
				"type":         "AD_GROUP",
				"action":       "CREATE",
				"reference_id": "ref_ad_group",
				"entity_data": map[string]interface{}{
					"entity_type":  "AD_GROUP",
					"name":         "Ad Group 1",
					"campaign_id":  "camp_123",
					"bid_strategy": "AUTO",
					"goal_type":    "CONVERSIONS",
					"start_time":   "2025-01-15T00:00:00Z",
					"targeting": map[string]interface{}{
						"geos": map[string]interface{}{
							"included": []map[string]interface{}{
								{"country": "US"},
							},
						},
						"devices": []string{"DESKTOP", "MOBILE"},
					},
				},
			},
			{
				"type":         "POST",
				"action":       "CREATE",
				"reference_id": "ref_post",
				"entity_data": map[string]interface{}{
					"entity_type": "POST",
					"headline":    "Amazing Product",
					"post_type":   "IMAGE",
					"content": []map[string]interface{}{
						{
							"content_type":    "IMAGE",
							"url":             "https://example.com/img.jpg",
							"destination_url": "https://example.com",
							"call_to_action":  "LEARN_MORE",
						},
					},
				},
			},
			{
				"type":         "AD",
				"action":       "CREATE",
				"reference_id": "ref_ad",
				"entity_data": map[string]interface{}{
					"entity_type": "AD",
					"name":        "Ad 1",
					"ad_group_id": "ag_123",
					"post_id":     "post_123",
					"click_url":   "https://example.com/landing",
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// Large payload: 20 items (5 of each entity type, simulates heavy BulkActions)
func largeMixedBulkActions() []byte {
	actions := make([]map[string]interface{}, 0, 20)

	for i := 0; i < 4; i++ {
		actions = append(actions, map[string]interface{}{
			"type":         "CAMPAIGN",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_campaign_%d", i),
			"entity_data": map[string]interface{}{
				"entity_type":           "CAMPAIGN",
				"name":                  fmt.Sprintf("Campaign %d", i),
				"objective":             "CONVERSIONS",
				"daily_budget_micro":    5000000,
				"start_time":            "2025-01-15T00:00:00Z",
				"funding_instrument_id": "fi_123",
			},
		})
		actions = append(actions, map[string]interface{}{
			"type":         "AD_GROUP",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_ag_%d", i),
			"entity_data": map[string]interface{}{
				"entity_type":  "AD_GROUP",
				"name":         fmt.Sprintf("Ad Group %d", i),
				"campaign_id":  "camp_123",
				"bid_strategy": "AUTO",
				"goal_type":    "CONVERSIONS",
				"start_time":   "2025-01-15T00:00:00Z",
			},
		})
		actions = append(actions, map[string]interface{}{
			"type":         "POST",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_post_%d", i),
			"entity_data": map[string]interface{}{
				"entity_type": "POST",
				"headline":    fmt.Sprintf("Product %d", i),
				"post_type":   "IMAGE",
				"content": []map[string]interface{}{
					{
						"content_type":    "IMAGE",
						"url":             "https://example.com/img.jpg",
						"destination_url": "https://example.com",
						"call_to_action":  "SHOP_NOW",
					},
				},
			},
		})
		actions = append(actions, map[string]interface{}{
			"type":         "AD",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_ad_%d", i),
			"entity_data": map[string]interface{}{
				"entity_type": "AD",
				"name":        fmt.Sprintf("Ad %d", i),
				"ad_group_id": "ag_123",
				"post_id":     "post_123",
				"click_url":   "https://example.com/landing",
			},
		})
		actions = append(actions, map[string]interface{}{
			"type":         "ASSET",
			"action":       "CREATE",
			"reference_id": fmt.Sprintf("ref_asset_%d", i),
			"entity_data": map[string]interface{}{
				"entity_type": "ASSET",
				"asset_type":  "IMAGE",
				"url":         "https://example.com/asset.jpg",
				"width":       1200,
				"height":      628,
			},
		})
	}

	payload := map[string]interface{}{"data": actions}
	b, _ := json.Marshal(payload)
	return b
}

// --- Benchmarks ---

// === Validator Init ===

func BenchmarkDiscriminator_Init_OneOf(b *testing.B) {
	loadDiscSpecs()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		doc, _ := libopenapi.NewDocument(oneOfSpec)
		v, errs := validator.NewValidator(doc)
		if errs != nil {
			b.Fatal(errs)
		}
		_ = v
	}
}

func BenchmarkDiscriminator_Init_IfThen(b *testing.B) {
	loadDiscSpecs()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		doc, _ := libopenapi.NewDocument(ifThenSpec)
		v, errs := validator.NewValidator(doc)
		if errs != nil {
			b.Fatal(errs)
		}
		_ = v
	}
}

// === Single item: CAMPAIGN (1st in oneOf list - best case for oneOf) ===

func BenchmarkDiscriminator_SingleCampaign_OneOf(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(oneOfDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := singleCampaignAction()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkDiscriminator_SingleCampaign_IfThen(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(ifThenDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := singleCampaignAction()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

// === Single item: POST (4th in oneOf list + nested content discriminator) ===

func BenchmarkDiscriminator_SinglePost_OneOf(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(oneOfDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := singlePostAction()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkDiscriminator_SinglePost_IfThen(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(ifThenDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := singlePostAction()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

// === Mixed payload: 4 items (typical BulkActions) ===

func BenchmarkDiscriminator_Mixed4_OneOf(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(oneOfDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := mixedBulkActions()
	b.ReportMetric(float64(len(payload)), "payload-bytes")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkDiscriminator_Mixed4_IfThen(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(ifThenDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := mixedBulkActions()
	b.ReportMetric(float64(len(payload)), "payload-bytes")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

// === Large payload: 20 items (heavy BulkActions) ===

func BenchmarkDiscriminator_Large20_OneOf(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(oneOfDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := largeMixedBulkActions()
	b.ReportMetric(float64(len(payload)), "payload-bytes")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

func BenchmarkDiscriminator_Large20_IfThen(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(ifThenDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := largeMixedBulkActions()
	b.ReportMetric(float64(len(payload)), "payload-bytes")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v3/ad_accounts/acc_123/bulk_actions",
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		v.ValidateHttpRequest(req)
	}
}

// === Concurrent mixed ===

func BenchmarkDiscriminator_Concurrent_OneOf(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(oneOfDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := mixedBulkActions()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest(http.MethodPost,
				"/api/v3/ad_accounts/acc_123/bulk_actions",
				bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			v.ValidateHttpRequest(req)
		}
	})
}

func BenchmarkDiscriminator_Concurrent_IfThen(b *testing.B) {
	buildDiscDocs()
	v, errs := validator.NewValidator(ifThenDoc)
	if errs != nil {
		b.Fatal(errs)
	}
	payload := mixedBulkActions()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest(http.MethodPost,
				"/api/v3/ad_accounts/acc_123/bulk_actions",
				bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			v.ValidateHttpRequest(req)
		}
	})
}
