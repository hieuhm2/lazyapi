package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ── OpenAPI struct definitions ────────────────────────────────────────────────

type openapiDoc struct {
	OpenAPI  string                        `json:"openapi"`  // 3.x
	Swagger  string                        `json:"swagger"`  // 2.x
	Info     openapiInfo                   `json:"info"`
	Servers  []openapiServer               `json:"servers"`   // 3.x
	Host     string                        `json:"host"`      // 2.x
	BasePath string                        `json:"basePath"`  // 2.x
	Schemes  []string                      `json:"schemes"`   // 2.x
	Paths    map[string]openapiPathItem    `json:"paths"`
	Components openapiComponents           `json:"components"` // 3.x
	Definitions map[string]openapiSchema   `json:"definitions"` // 2.x
}

type openapiInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type openapiServer struct {
	URL string `json:"url"`
}

// Path item: keys are http methods + optional "parameters", "summary", etc.
type openapiPathItem map[string]json.RawMessage

type openapiOperation struct {
	Summary     string              `json:"summary"`
	OperationID string              `json:"operationId"`
	Description string              `json:"description"`
	Tags        []string            `json:"tags"`
	Parameters  []openapiParameter  `json:"parameters"`
	RequestBody *openapiRequestBody `json:"requestBody"` // 3.x
	Consumes    []string            `json:"consumes"`    // 2.x
}

type openapiParameter struct {
	Name     string        `json:"name"`
	In       string        `json:"in"`
	Required bool          `json:"required"`
	Schema   *openapiSchema `json:"schema"`
	Type     string        `json:"type"` // 2.x inline
}

type openapiRequestBody struct {
	Content map[string]openapiMediaType `json:"content"`
}

type openapiMediaType struct {
	Schema  *openapiSchema  `json:"schema"`
	Example json.RawMessage `json:"example"`
}

type openapiSchema struct {
	Ref        string                    `json:"$ref"`
	Type       string                    `json:"type"`
	Format     string                    `json:"format"`
	Properties map[string]*openapiSchema `json:"properties"`
	Items      *openapiSchema            `json:"items"`
	Example    json.RawMessage           `json:"example"`
	Enum       []interface{}             `json:"enum"`
	Required   []string                  `json:"required"`
	AllOf      []*openapiSchema          `json:"allOf"`
}

type openapiComponents struct {
	Schemas map[string]*openapiSchema `json:"schemas"`
}

// ── Public import function ────────────────────────────────────────────────────

// ImportOpenAPI parses an OpenAPI 3.x or Swagger 2.x file and returns collections.
func ImportOpenAPI(filePath string) ([]Collection, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var doc openapiDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("no paths found in file — is this a valid OpenAPI/Swagger document?")
	}

	title := doc.Info.Title
	if title == "" {
		title = "Imported API"
	}

	baseURL := resolveBaseURL(doc)

	// Collect all schemas for $ref resolution
	schemas := map[string]*openapiSchema{}
	for k, v := range doc.Components.Schemas {
		s := v
		schemas["#/components/schemas/"+k] = s
	}
	for k, v := range doc.Definitions {
		s := v
		schemas["#/definitions/"+k] = &s
	}

	// Group requests by first tag
	tagRequests := map[string][]Request{}
	var tagOrder []string
	tagSeen := map[string]bool{}

	httpMethods := []string{"get", "post", "put", "patch", "delete", "head", "options"}

	// Sort paths for deterministic output
	sortedPaths := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	for _, path := range sortedPaths {
		pathItem := doc.Paths[path]
		for _, method := range httpMethods {
			raw, ok := pathItem[method]
			if !ok {
				continue
			}
			var op openapiOperation
			if err := json.Unmarshal(raw, &op); err != nil {
				continue
			}

			req := buildRequest(method, path, baseURL, op, schemas)

			tag := title // default collection = API title
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			if !tagSeen[tag] {
				tagSeen[tag] = true
				tagOrder = append(tagOrder, tag)
			}
			tagRequests[tag] = append(tagRequests[tag], req)
		}
	}

	var collections []Collection
	for _, tag := range tagOrder {
		colName := tag
		if tag != title {
			colName = title + " › " + tag
		}
		collections = append(collections, Collection{
			ID:       NewID(),
			Name:     colName,
			Requests: tagRequests[tag],
		})
	}

	return collections, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func resolveBaseURL(doc openapiDoc) string {
	// OpenAPI 3.x
	if len(doc.Servers) > 0 {
		return strings.TrimRight(doc.Servers[0].URL, "/")
	}
	// Swagger 2.x
	if doc.Host != "" {
		scheme := "https"
		if len(doc.Schemes) > 0 {
			scheme = doc.Schemes[0]
		}
		return scheme + "://" + doc.Host + doc.BasePath
	}
	return ""
}

func buildRequest(method, path, baseURL string, op openapiOperation, schemas map[string]*openapiSchema) Request {
	// Name: prefer operationId → summary → METHOD /path
	name := op.OperationID
	if name == "" {
		name = op.Summary
	}
	if name == "" {
		name = strings.ToUpper(method) + " " + path
	}

	// Build URL, append query params
	url := baseURL + path
	var queryParams []string
	for _, p := range op.Parameters {
		if p.In == "query" {
			queryParams = append(queryParams, p.Name+"=")
		}
	}
	if len(queryParams) > 0 {
		sort.Strings(queryParams)
		url += "?" + strings.Join(queryParams, "&")
	}

	headers := map[string]string{}
	body := ""

	// OpenAPI 3.x requestBody
	if op.RequestBody != nil {
		if mt, ok := op.RequestBody.Content["application/json"]; ok {
			headers["Content-Type"] = "application/json"
			body = extractBody(mt, schemas)
		} else if _, ok := op.RequestBody.Content["multipart/form-data"]; ok {
			headers["Content-Type"] = "multipart/form-data"
		} else if _, ok := op.RequestBody.Content["application/x-www-form-urlencoded"]; ok {
			headers["Content-Type"] = "application/x-www-form-urlencoded"
		} else {
			// Take first content type
			for ct, mt := range op.RequestBody.Content {
				headers["Content-Type"] = ct
				body = extractBody(mt, schemas)
				break
			}
		}
	}

	// Swagger 2.x: body parameter
	for _, p := range op.Parameters {
		if p.In == "body" && p.Schema != nil {
			headers["Content-Type"] = "application/json"
			example := generateExample(resolveRef(p.Schema, schemas), schemas, 0)
			if b, err := prettyJSON(example); err == nil {
				body = b
			}
			break
		}
	}

	return Request{
		ID:      NewID(),
		Name:    name,
		Method:  strings.ToUpper(method),
		URL:     url,
		Headers: headers,
		Body:    body,
	}
}

func extractBody(mt openapiMediaType, schemas map[string]*openapiSchema) string {
	// Prefer explicit example
	if mt.Example != nil && string(mt.Example) != "null" {
		if b, err := prettyJSONRaw(mt.Example); err == nil {
			return b
		}
	}
	// Generate from schema
	if mt.Schema != nil {
		example := generateExample(resolveRef(mt.Schema, schemas), schemas, 0)
		if b, err := prettyJSON(example); err == nil {
			return b
		}
	}
	return ""
}

// resolveRef follows a $ref pointer if present.
func resolveRef(s *openapiSchema, schemas map[string]*openapiSchema) *openapiSchema {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		if resolved, ok := schemas[s.Ref]; ok {
			return resolved
		}
	}
	return s
}

// generateExample produces a sample value from a schema (max depth 5).
func generateExample(s *openapiSchema, schemas map[string]*openapiSchema, depth int) interface{} {
	if s == nil || depth > 5 {
		return nil
	}
	s = resolveRef(s, schemas)
	if s == nil {
		return nil
	}

	// Explicit example wins
	if s.Example != nil && string(s.Example) != "null" {
		var v interface{}
		if json.Unmarshal(s.Example, &v) == nil {
			return v
		}
	}
	// Enum: pick first
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	// allOf: merge all schemas
	if len(s.AllOf) > 0 {
		merged := map[string]interface{}{}
		for _, sub := range s.AllOf {
			sub = resolveRef(sub, schemas)
			if sub == nil {
				continue
			}
			if v, ok := generateExample(sub, schemas, depth+1).(map[string]interface{}); ok {
				for k, val := range v {
					merged[k] = val
				}
			}
		}
		if len(merged) > 0 {
			return merged
		}
	}

	switch s.Type {
	case "object", "":
		if len(s.Properties) == 0 {
			return map[string]interface{}{}
		}
		obj := map[string]interface{}{}
		// Sort property keys for determinism
		keys := make([]string, 0, len(s.Properties))
		for k := range s.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			obj[k] = generateExample(s.Properties[k], schemas, depth+1)
		}
		return obj
	case "array":
		if s.Items != nil {
			return []interface{}{generateExample(s.Items, schemas, depth+1)}
		}
		return []interface{}{}
	case "string":
		switch s.Format {
		case "date":
			return "2024-01-01"
		case "date-time":
			return "2024-01-01T00:00:00Z"
		case "email":
			return "user@example.com"
		case "uuid":
			return "00000000-0000-0000-0000-000000000000"
		case "uri", "url":
			return "https://example.com"
		case "binary", "byte":
			return ""
		default:
			return "string"
		}
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return true
	case "null":
		return nil
	}
	return nil
}

func prettyJSON(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func prettyJSONRaw(raw json.RawMessage) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}
