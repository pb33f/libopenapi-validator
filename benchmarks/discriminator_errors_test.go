// Compare validation error messages between oneOf+discriminator and if/then approaches.
//
// Run with:
//   go test -run=TestErrorComparison -v -timeout=5m ./benchmarks/

package benchmarks

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"

	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/errors"
)

func loadDiscriminatorSpecs(t *testing.T) (validator.Validator, validator.Validator) {
	t.Helper()

	oneOfBytes, err := os.ReadFile("../test_specs/discriminator_oneof.yaml")
	if err != nil {
		t.Fatalf("failed to read discriminator_oneof.yaml: %v", err)
	}
	ifThenBytes, err := os.ReadFile("../test_specs/discriminator_ifthen.yaml")
	if err != nil {
		t.Fatalf("failed to read discriminator_ifthen.yaml: %v", err)
	}

	oneOfDoc, err := libopenapi.NewDocument(oneOfBytes)
	if err != nil {
		t.Fatalf("failed to parse oneOf spec: %v", err)
	}
	ifThenDoc, err := libopenapi.NewDocument(ifThenBytes)
	if err != nil {
		t.Fatalf("failed to parse if/then spec: %v", err)
	}

	oneOfV, errs := validator.NewValidator(oneOfDoc)
	if errs != nil {
		t.Fatalf("failed to create oneOf validator: %v", errs)
	}
	ifThenV, errs := validator.NewValidator(ifThenDoc)
	if errs != nil {
		t.Fatalf("failed to create if/then validator: %v", errs)
	}

	return oneOfV, ifThenV
}

func makeDiscReq(t *testing.T, payload interface{}) *http.Request {
	t.Helper()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_123/bulk_actions",
		bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func printErrors(t *testing.T, label string, valid bool, errs []*errors.ValidationError) {
	t.Helper()
	t.Logf("\n=== %s ===", label)
	t.Logf("Valid: %v | Error count: %d", valid, len(errs))
	for i, e := range errs {
		t.Logf("  [%d] Message: %s", i+1, e.Message)
		if e.Reason != "" {
			t.Logf("       Reason: %s", e.Reason)
		}
		for j, sve := range e.SchemaValidationErrors {
			t.Logf("       schema[%d] reason: %s", j+1, sve.Reason)
			if sve.Location != "" {
				t.Logf("       schema[%d] location: %s", j+1, sve.Location)
			}
			if sve.FieldPath != "" {
				t.Logf("       schema[%d] fieldPath: %s", j+1, sve.FieldPath)
			}
		}
	}
}

// --- Test Cases ---

// Case 1: Valid CAMPAIGN payload (should pass both)
func TestErrorComparison_ValidCampaign(t *testing.T) {
	oneOfV, ifThenV := loadDiscriminatorSpecs(t)

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
				},
			},
		},
	}

	req1 := makeDiscReq(t, payload)
	valid1, errs1 := oneOfV.ValidateHttpRequest(req1)

	req2 := makeDiscReq(t, payload)
	valid2, errs2 := ifThenV.ValidateHttpRequest(req2)

	printErrors(t, "oneOf: Valid Campaign", valid1, errs1)
	printErrors(t, "if/then: Valid Campaign", valid2, errs2)
}

// Case 2: Missing required fields in entity_data (CAMPAIGN missing name + objective)
func TestErrorComparison_MissingRequired(t *testing.T) {
	oneOfV, ifThenV := loadDiscriminatorSpecs(t)

	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_1",
				"entity_data": map[string]interface{}{
					"entity_type":        "CAMPAIGN",
					"daily_budget_micro": 5000000,
					// missing: name, objective, start_time, funding_instrument_id
				},
			},
		},
	}

	req1 := makeDiscReq(t, payload)
	valid1, errs1 := oneOfV.ValidateHttpRequest(req1)

	req2 := makeDiscReq(t, payload)
	valid2, errs2 := ifThenV.ValidateHttpRequest(req2)

	printErrors(t, "oneOf: Campaign missing required fields", valid1, errs1)
	printErrors(t, "if/then: Campaign missing required fields", valid2, errs2)
}

// Case 3: Invalid field value (bad enum for objective)
func TestErrorComparison_InvalidEnum(t *testing.T) {
	oneOfV, ifThenV := loadDiscriminatorSpecs(t)

	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_1",
				"entity_data": map[string]interface{}{
					"entity_type":           "CAMPAIGN",
					"name":                  "Test Campaign",
					"objective":             "NOT_A_VALID_OBJECTIVE",
					"daily_budget_micro":    5000000,
					"start_time":            "2025-01-15T00:00:00Z",
					"funding_instrument_id": "fi_123",
				},
			},
		},
	}

	req1 := makeDiscReq(t, payload)
	valid1, errs1 := oneOfV.ValidateHttpRequest(req1)

	req2 := makeDiscReq(t, payload)
	valid2, errs2 := ifThenV.ValidateHttpRequest(req2)

	printErrors(t, "oneOf: Invalid objective enum", valid1, errs1)
	printErrors(t, "if/then: Invalid objective enum", valid2, errs2)
}

// Case 4: POST action with nested content error (VIDEO missing required "url")
func TestErrorComparison_NestedDiscriminatorError(t *testing.T) {
	oneOfV, ifThenV := loadDiscriminatorSpecs(t)

	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "POST",
				"action":       "CREATE",
				"reference_id": "ref_1",
				"entity_data": map[string]interface{}{
					"entity_type": "POST",
					"headline":    "Check it out!",
					"post_type":   "IMAGE",
					"content": []map[string]interface{}{
						{
							"content_type": "IMAGE",
							"url":          "https://example.com/img.jpg",
						},
						{
							"content_type": "VIDEO",
							// Missing required "url" field
						},
					},
				},
			},
		},
	}

	req1 := makeDiscReq(t, payload)
	valid1, errs1 := oneOfV.ValidateHttpRequest(req1)

	req2 := makeDiscReq(t, payload)
	valid2, errs2 := ifThenV.ValidateHttpRequest(req2)

	printErrors(t, "oneOf: Nested content error (VIDEO missing url)", valid1, errs1)
	printErrors(t, "if/then: Nested content error (VIDEO missing url)", valid2, errs2)
}

// Case 5: Completely invalid entity_type value
func TestErrorComparison_UnknownEntityType(t *testing.T) {
	oneOfV, ifThenV := loadDiscriminatorSpecs(t)

	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_1",
				"entity_data": map[string]interface{}{
					"entity_type": "BANANA",
					"name":        "Test",
				},
			},
		},
	}

	req1 := makeDiscReq(t, payload)
	valid1, errs1 := oneOfV.ValidateHttpRequest(req1)

	req2 := makeDiscReq(t, payload)
	valid2, errs2 := ifThenV.ValidateHttpRequest(req2)

	printErrors(t, "oneOf: Unknown entity_type 'BANANA'", valid1, errs1)
	printErrors(t, "if/then: Unknown entity_type 'BANANA'", valid2, errs2)
}

// Case 6: Multiple items where some valid, some invalid
func TestErrorComparison_MixedValidInvalid(t *testing.T) {
	oneOfV, ifThenV := loadDiscriminatorSpecs(t)

	payload := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{
				"type":         "CAMPAIGN",
				"action":       "CREATE",
				"reference_id": "ref_ok",
				"entity_data": map[string]interface{}{
					"entity_type":           "CAMPAIGN",
					"name":                  "Good Campaign",
					"objective":             "CONVERSIONS",
					"daily_budget_micro":    5000000,
					"start_time":            "2025-01-15T00:00:00Z",
					"funding_instrument_id": "fi_123",
				},
			},
			map[string]interface{}{
				"type":         "AD",
				"action":       "CREATE",
				"reference_id": "ref_bad",
				"entity_data": map[string]interface{}{
					"entity_type": "AD",
					// missing: name, ad_group_id, post_id
				},
			},
		},
	}

	req1 := makeDiscReq(t, payload)
	valid1, errs1 := oneOfV.ValidateHttpRequest(req1)

	req2 := makeDiscReq(t, payload)
	valid2, errs2 := ifThenV.ValidateHttpRequest(req2)

	printErrors(t, "oneOf: Mixed valid+invalid items", valid1, errs1)
	printErrors(t, "if/then: Mixed valid+invalid items", valid2, errs2)
}
