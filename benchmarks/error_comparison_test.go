// Compare validation error messages using the production complete.yaml spec.
//
// Run with:
//   go test -run=TestProdErrors -v -timeout=5m ./benchmarks/

package benchmarks

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/pb33f/libopenapi-validator/errors"
)

func printDetailedErrors(t *testing.T, label string, valid bool, errs []*errors.ValidationError) {
	t.Helper()
	t.Logf("\n=== %s ===", label)
	t.Logf("Valid: %v | Error count: %d", valid, len(errs))
	for i, e := range errs {
		t.Logf("  [%d] Message: %s", i+1, e.Message)
		if e.Reason != "" {
			t.Logf("       Reason: %s", e.Reason)
		}
		if e.ValidationType != "" {
			t.Logf("       ValidationType: %s", e.ValidationType)
		}
		for j, sve := range e.SchemaValidationErrors {
			t.Logf("       schema[%d] reason: %s", j+1, sve.Reason)
			if sve.Location != "" {
				t.Logf("       schema[%d] location: %s", j+1, sve.Location)
			}
			if sve.FieldPath != "" {
				t.Logf("       schema[%d] fieldPath: %s", j+1, sve.FieldPath)
			}
			if sve.FieldName != "" {
				t.Logf("       schema[%d] fieldName: %s", j+1, sve.FieldName)
			}
		}
	}
}

func makeProdReq(t *testing.T, payload interface{}) *http.Request {
	t.Helper()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost,
		"/api/v3/ad_accounts/acc_12345/bulk_actions",
		bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// Case 1: Missing required field (entity_data)
func TestProdErrors_MissingEntityData(t *testing.T) {
	v := newProdValidator(t)
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "CAMPAIGN",
				"action": "CREATE",
				// entity_data is missing
			},
		},
	}
	req := makeProdReq(t, payload)
	valid, errs := v.ValidateHttpRequest(req)
	printDetailedErrors(t, "Missing entity_data", valid, errs)
}

// Case 2: Invalid enum value for "type"
func TestProdErrors_InvalidType(t *testing.T) {
	v := newProdValidator(t)
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "BANANA",
				"action": "CREATE",
				"entity_data": map[string]interface{}{
					"name":      "Test",
					"objective": "CONVERSIONS",
				},
			},
		},
	}
	req := makeProdReq(t, payload)
	valid, errs := v.ValidateHttpRequest(req)
	printDetailedErrors(t, "Invalid type 'BANANA'", valid, errs)
}

// Case 3: Extra field on BulkAction item (additionalProperties: false)
func TestProdErrors_ExtraField(t *testing.T) {
	v := newProdValidator(t)
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
	req := makeProdReq(t, payload)
	valid, errs := v.ValidateHttpRequest(req)
	printDetailedErrors(t, "Extra field on item", valid, errs)
}

// Case 4: Invalid objective enum
func TestProdErrors_InvalidEnum(t *testing.T) {
	v := newProdValidator(t)
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "CAMPAIGN",
				"action": "CREATE",
				"entity_data": map[string]interface{}{
					"name":      "Campaign",
					"objective": "NOT_REAL",
				},
			},
		},
	}
	req := makeProdReq(t, payload)
	valid, errs := v.ValidateHttpRequest(req)
	printDetailedErrors(t, "Invalid objective enum", valid, errs)
}

// Case 5: Mixed valid + invalid items
func TestProdErrors_MixedItems(t *testing.T) {
	v := newProdValidator(t)
	payload := map[string]interface{}{
		"data": []interface{}{
			// Valid campaign
			map[string]interface{}{
				"type":   "CAMPAIGN",
				"action": "CREATE",
				"entity_data": map[string]interface{}{
					"name":      "Good Campaign",
					"objective": "CONVERSIONS",
				},
			},
			// Invalid - missing entity_data
			map[string]interface{}{
				"type":   "AD_GROUP",
				"action": "CREATE",
				// missing entity_data
			},
		},
	}
	req := makeProdReq(t, payload)
	valid, errs := v.ValidateHttpRequest(req)
	printDetailedErrors(t, "Mixed valid + invalid", valid, errs)
}

// Case 6: Wrong action for POST (POST only allows CREATE in the if/then spec)
func TestProdErrors_WrongActionForPost(t *testing.T) {
	v := newProdValidator(t)
	payload := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"type":   "POST",
				"action": "EDIT",
				"entity_data": map[string]interface{}{
					"creative": map[string]interface{}{
						"type":     "IMAGE",
						"headline": "Test",
					},
				},
			},
		},
	}
	req := makeProdReq(t, payload)
	valid, errs := v.ValidateHttpRequest(req)
	printDetailedErrors(t, "POST with action=EDIT (only CREATE allowed)", valid, errs)
}
