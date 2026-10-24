package inference

import (
	"encoding/json"
	"reflect"
	"testing"
)

type diagnosisTestSchema struct {
	Type       string                         `json:"type"`
	Properties map[string]diagnosisTestSchema `json:"properties"`
	Items      *diagnosisTestSchema           `json:"items"`
}

func TestDiagnosisResultMatchesSchema(t *testing.T) {
	var wrapper struct {
		Schema diagnosisTestSchema `json:"schema"`
	}
	if err := json.Unmarshal(diagnosisSchemaBytes, &wrapper); err != nil {
		t.Fatal(err)
	}
	var check func(reflect.Type, diagnosisTestSchema)
	check = func(typ reflect.Type, schema diagnosisTestSchema) {
		t.Helper()
		switch schema.Type {
		case "object":
			if typ.Kind() != reflect.Struct || typ.NumField() != len(schema.Properties) {
				t.Fatalf("%v does not match schema object", typ)
			}
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				name := field.Tag.Get("json")
				property, ok := schema.Properties[name]
				if !ok {
					t.Fatalf("field %s has no schema property %q", field.Name, name)
				}
				check(field.Type, property)
			}
		case "array":
			if typ.Kind() != reflect.Slice || schema.Items == nil {
				t.Fatalf("%v does not match schema array", typ)
			}
			check(typ.Elem(), *schema.Items)
		case "string":
			if typ.Kind() != reflect.String {
				t.Fatalf("%v does not match schema string", typ)
			}
		case "integer":
			if typ.Kind() != reflect.Int64 {
				t.Fatalf("%v does not match schema integer", typ)
			}
		default:
			t.Fatalf("unsupported schema type %q", schema.Type)
		}
	}
	check(reflect.TypeFor[DiagnosisResult](), wrapper.Schema)
}

func TestDiagnosisJSONRoundTrip(t *testing.T) {
	input := []byte(`{
		"occurredAt":"2026-10-04T12:00:00Z",
		"title":"Database unavailable",
		"summary":"Checkout failed.",
		"rootCause":"Database connections exhausted.",
		"possibleContributingFactors":["Connection leak"],
		"timeline":[{"logIds":["log-1"],"event":"Connections exhausted"}],
		"severity":0,
		"recommendedActions":[{"type":"mitigation","action":"Restart service"}]
	}`)
	var result DiagnosisResult
	if err := json.Unmarshal(input, &result); err != nil {
		t.Fatal(err)
	}
	if result.OccurredAt != "2026-10-04T12:00:00Z" ||
		result.RootCause != "Database connections exhausted." ||
		len(result.RecommendedActions) != 1 || result.RecommendedActions[0].Type != "mitigation" {
		t.Fatal("schema fields were not decoded correctly")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual map[string]any
	if err := json.Unmarshal(input, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal("diagnosis lost fields during round trip")
	}
}

func TestParseDiagnosisResult(t *testing.T) {
	valid := `{
			"occurredAt":"2026-10-04T12:00:00Z",
			"title":"Failure","summary":"Summary","rootCause":"Cause",
			"possibleContributingFactors":[],
			"timeline":[{"logIds":["log-1"],"event":"Failure"}],
			"severity":0,
			"recommendedActions":[{"type":"mitigation","action":"Restart"}]
		}`
	result, err := ParseDiagnosisResult([]byte(valid))
	if err != nil {
		t.Fatalf("valid diagnosis rejected: %v", err)
	}
	if result.Title != "Failure" || result.RecommendedActions[0].Type != Mitigation {
		t.Fatalf("unexpected decode result: %+v", result)
	}
	if _, err := ParseDiagnosisResult([]byte("{")); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}
