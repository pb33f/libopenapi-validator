# BulkAction Schema: oneOf to if/then Migration

## Problem

The BulkAction schema uses `oneOf` with 5 branches (CAMPAIGN, AD_GROUP, AD, POST, ASSET).
JSON Schema `oneOf` validates the input against **every** branch to determine which one matches.
This means a single bulk action item triggers 5 full schema validations even though only one can match.

This has two consequences:
1. **Performance**: validation does 5x the work needed on every request.
2. **Error noise**: when validation fails, errors from all 5 branches are returned — most of which are irrelevant.

## Change

Replace `oneOf` in `components_schema_bulk_actions_bulk_action.yaml` with `allOf` + `if/then` keyed on the `type` field.

Before:

```yaml
title: Bulk Action
oneOf:
  - $ref: "components_schema_bulk_actions__campaign_action.yaml"
  - $ref: "components_schema_bulk_actions__ad_group_action.yaml"
  - $ref: "components_schema_bulk_actions__ad_action.yaml"
  - $ref: "components_schema_bulk_actions__post_action.yaml"
  - $ref: components_schema_bulk_actions__asset_action.yaml
```

After:

```yaml
title: Bulk Action
type: object
additionalProperties: false
required: [type, action, entity_data]
properties:
  type:
    $ref: components_enums_bulk_actions_entity_type.yaml
  action:
    $ref: components_enums_bulk_actions_action_type.yaml
  reference_id:
    type: string
  entity_data: {}
allOf:
  - if: { required: [type], properties: { type: { const: CAMPAIGN } } }
    then: { properties: { entity_data: { anyOf: [{ $ref: components_schema_campaign.yaml }, { type: object, additionalProperties: true }] } } }
  - if: { required: [type], properties: { type: { const: AD_GROUP } } }
    then: { properties: { entity_data: { anyOf: [{ $ref: components_schema_ad_group.yaml }, { type: object, additionalProperties: true }] } } }
  # ... AD, POST, ASSET follow the same pattern
```

Only `bulk_action.yaml` changes. The individual action type schemas are untouched.

## Benchmark Results

Tested against the full production `complete.yaml` (68k lines, ~200 endpoints).
Apple M4 Max, `go test -bench=BenchmarkProd -benchmem -count=3`.

### BulkActions Validation

| Benchmark | oneOf ns/op | if/then ns/op | Speedup | oneOf B/op | if/then B/op | Mem Reduction |
|---|---|---|---|---|---|---|
| SingleCampaign | 25,560 | 21,070 | 1.2x | 17,498 | 13,072 | -25% |
| Mixed (3 items) | 51,770 | 40,320 | 1.3x | 53,129 | 38,538 | -27% |
| Large (15 items) | 177,460 | 128,300 | 1.4x | 244,602 | 172,368 | -30% |
| Invalid: missing field | 81,460 | 21,400 | **3.8x** | 87,740 | 17,687 | **-80%** |
| Invalid: wrong type | 123,080 | 24,550 | **5.0x** | 141,487 | 20,313 | **-86%** |
| Invalid: extra field | 151,340 | 28,540 | **5.3x** | 173,099 | 19,714 | **-89%** |
| Sync | 35,980 | 26,540 | 1.4x | 51,692 | 37,025 | -28% |
| Async | 50,200 | 41,200 | 1.2x | 53,147 | 38,538 | -27% |
| Concurrent | 10,960 | 8,120 | 1.3x | 53,138 | 38,535 | -27% |

### Geomean (all benchmarks)

| Metric | Improvement |
|---|---|
| Time (ns/op) | **-31%** |
| Memory (B/op) | **-37%** |
| Allocations | **-38%** |

### Non-BulkAction endpoints

GET endpoints (campaigns, ad groups, ads, /me) were unchanged or slightly faster (~5-6% less memory). Init time was the same (~2.7s).

### Why invalid payloads improve the most

With `oneOf`, an invalid payload fails validation against all 5 branches, and the library collects errors from each one. With `if/then`, only the matching branch is evaluated — if `type=CAMPAIGN`, the AD_GROUP/AD/POST/ASSET branches are never touched.

## Error Quality

Tested against the production `complete.yaml` with both schemas. All tests use the `/api/v3/ad_accounts/{id}/bulk_actions` POST endpoint.

### Missing entity_data

**oneOf** — 9 schema errors:

```
schema[1] reason: missing property 'entity_data'       (oneOf/0 - Campaign)
schema[2] reason: missing property 'entity_data'       (oneOf/1 - AdGroup)
schema[3] reason: value must be 'AD_GROUP'              (noise)
schema[4] reason: missing property 'entity_data'       (oneOf/2 - Ad)
schema[5] reason: value must be 'AD'                    (noise)
schema[6] reason: missing property 'entity_data'       (oneOf/3 - Post)
schema[7] reason: value must be 'POST'                  (noise)
schema[8] reason: missing property 'entity_data'       (oneOf/4 - Asset)
schema[9] reason: value must be 'ASSET'                 (noise)
```

**if/then** — 1 schema error:

```
schema[1] reason: missing property 'entity_data'
         location: /properties/data/items/required
         fieldPath: $.data[0]
```

### Invalid type "BANANA"

**oneOf** — 11 schema errors: "value must be 'CAMPAIGN'", "value must be 'AD_GROUP'", etc. from each branch, plus cascading errors from ASSET/POST entity_data schemas that don't apply.

**if/then** — 1 schema error:

```
schema[1] reason: value must be one of 'CAMPAIGN', 'AD_GROUP', 'AD', 'POST', 'ASSET'
         location: /properties/data/items/properties/type/enum
         fieldPath: $.data[0].type
```

### Extra field (additionalProperties: false)

**oneOf** — 11 schema errors: "additional properties not allowed" repeated 5 times (once per branch), plus "value must be 'AD_GROUP'" etc. for every non-matching branch.

**if/then** — 1 schema error:

```
schema[1] reason: additional properties 'this_field_does_not_exist' not allowed
         location: /properties/data/items/additionalProperties
         fieldPath: $.data[0]
```

### POST with action=EDIT (only CREATE allowed)

**oneOf** — 28 schema errors: errors from all 5 branches including CAMPAIGN, AD_GROUP, AD, ASSET — none relevant since type=POST.

**if/then** — 18 schema errors: `value must be 'CREATE'` once at `$.data[0].action`, plus errors from the POST branch's inner `creative` oneOf (which still uses oneOf in both specs — a candidate for the same treatment).

### Invalid objective enum

Both return valid with 0 errors. The `entity_data` for CAMPAIGN uses `anyOf` (strict schema OR permissive object), so bad fields in entity_data pass through the permissive branch. Same behavior either way — not a regression.

## Summary

- **Only 1 file changed** in the OpenAPI source: `components_schema_bulk_actions_bulk_action.yaml`
- **1.2-5.3x faster**, biggest gains on invalid payloads (the most common case in production)
- **25-89% less memory** per validation
- **Dramatically fewer error messages** with better signal-to-noise ratio
- **No functional regressions**: all existing valid/invalid test payloads behave identically
- **No init time penalty**: spec compilation takes the same ~2.7s
- The structured_post `creative` field still uses `oneOf` internally — applying the same if/then pattern there would yield further improvements
