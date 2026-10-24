// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    generatedDiagnosis, err := UnmarshalGeneratedDiagnosis(bytes)
//    bytes, err = generatedDiagnosis.Marshal()

package diagnosisschema

import "encoding/json"

func UnmarshalGeneratedDiagnosis(data []byte) (GeneratedDiagnosis, error) {
	var r GeneratedDiagnosis
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GeneratedDiagnosis) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type GeneratedDiagnosis struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Strict      bool   `json:"strict"`
	Schema      Schema `json:"schema"`
}

type Schema struct {
	Type                 string           `json:"type"`
	AdditionalProperties bool             `json:"additionalProperties"`
	Properties           SchemaProperties `json:"properties"`
	Required             []string         `json:"required"`
}

type SchemaProperties struct {
	OccurredAt                  OccurredAt                  `json:"occurredAt"`
	Title                       RootCause                   `json:"title"`
	Summary                     RootCause                   `json:"summary"`
	RootCause                   RootCause                   `json:"rootCause"`
	PossibleContributingFactors PossibleContributingFactors `json:"possibleContributingFactors"`
	Timeline                    Timeline                    `json:"timeline"`
	Severity                    Severity                    `json:"severity"`
	RecommendedActions          RecommendedActions          `json:"recommendedActions"`
}

type OccurredAt struct {
	Type        string `json:"type"`
	Format      string `json:"format"`
	Description string `json:"description"`
}

type PossibleContributingFactors struct {
	Type        string                           `json:"type"`
	Items       PossibleContributingFactorsItems `json:"items"`
	Description string                           `json:"description"`
}

type PossibleContributingFactorsItems struct {
	Type string `json:"type"`
}

type RecommendedActions struct {
	Type        string                  `json:"type"`
	Items       RecommendedActionsItems `json:"items"`
	Description string                  `json:"description"`
}

type RecommendedActionsItems struct {
	Type                 string           `json:"type"`
	AdditionalProperties bool             `json:"additionalProperties"`
	Properties           PurpleProperties `json:"properties"`
	Required             []string         `json:"required"`
}

type PurpleProperties struct {
	Type   Type      `json:"type"`
	Action RootCause `json:"action"`
}

type RootCause struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Type struct {
	Type string   `json:"type"`
	Enum []string `json:"enum"`
}

type Severity struct {
	Type        string `json:"type"`
	Minimum     int64  `json:"minimum"`
	Maximum     int64  `json:"maximum"`
	Description string `json:"description"`
}

type Timeline struct {
	Type        string        `json:"type"`
	Items       TimelineItems `json:"items"`
	Description string        `json:"description"`
}

type TimelineItems struct {
	Type                 string           `json:"type"`
	AdditionalProperties bool             `json:"additionalProperties"`
	Properties           FluffyProperties `json:"properties"`
	Required             []string         `json:"required"`
}

type FluffyProperties struct {
	LogIDS PossibleContributingFactors `json:"logIds"`
	Event  RootCause                   `json:"event"`
}
