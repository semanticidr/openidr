# Diagnosis output

`diagnosis_result.json` is the source of truth for both provider schemas and
diagnosis output types. The inference model remains separate from the incident
protobuf model.

Install Node.js and npm, then run `npm install -g quicktype` and ensure
`quicktype` is available on `PATH`. The generator checks for it before reading
input or creating files and fails with installation guidance if it is missing.

With Quicktype available on `PATH`, run `go generate ./inference` from the
repository root. This generates:

- `internal/diagnosisschema/generated_diagnosis.go`: the schema-document model.
- `generated_result.go`: the output model, generated from the inner JSON Schema.

Do not edit generated files. The generator extracts the inner schema into a
temporary file and removes it afterward.

Both LLM clients use `ParseDiagnosisResult`, which unmarshals the raw JSON into
`DiagnosisResult` and then calls `DiagnosisResult.Validate` for application-level
checks.

## Provider authentication

Provider credentials are deployment configuration. The application enables providers through 
`LLM_PROVIDERS`; each provider client reads its own environment settings.
At startup, enabled clients check readiness with a bounded model-list request and prevent the service
from starting if authentication or provider access is unavailable.

OpenAI uses `OPENAI_API_KEY`. Gemini requires an explicit `GEMINI_AUTH_MODE`: `api_key` uses
`GEMINI_API_KEY`, while `vertex` uses `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION`, and Google
Application Default Credentials.
