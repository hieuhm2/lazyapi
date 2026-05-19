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
	OpenAPI    string                     `json:"openapi"`
	Swagger    string                     `json:"swagger"`
	Info       openapiInfo                `json:"info"`
	Servers    []openapiServer            `json:"servers"`
	Host       string                     `json:"host"`
	BasePath   string                     `json:"basePath"`
	Schemes    []string                   `json:"schemes"`
	Paths      map[string]openapiPathItem `json:"paths"`
	Components openapiComponents          `json:"components"`
	Definitions map[string]openapiSchema  `json:"definitions"`
}

type openapiInfo   struct{ Title, Version string }
type openapiServer struct{ URL string }

type openapiPathItem map[string]json.RawMessage

type openapiOperation struct {
	Summary     string             `json:"summary"`
	OperationID string             `json:"operationId"`
	Tags        []string           `json:"tags"`
	Parameters  []openapiParameter `json:"parameters"`
	RequestBody *openapiReqBody    `json:"requestBody"`
}

type openapiParameter struct {
	Name   string         `json:"name"`
	In     string         `json:"in"`
	Schema *openapiSchema `json:"schema"`
}

type openapiReqBody struct {
	Content map[string]openapiMedia `json:"content"`
}

type openapiMedia struct {
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
	AllOf      []*openapiSchema          `json:"allOf"`
}

type openapiComponents struct {
	Schemas map[string]*openapiSchema `json:"schemas"`
}

// ── Path trie ─────────────────────────────────────────────────────────────────

type pathTrie struct {
	name       string
	requests   []Request
	children   map[string]*pathTrie
	childOrder []string
}

func newTrie(name string) *pathTrie {
	return &pathTrie{name: name, children: map[string]*pathTrie{}}
}

func (t *pathTrie) child(seg string) *pathTrie {
	if _, ok := t.children[seg]; !ok {
		t.children[seg] = newTrie(seg)
		t.childOrder = append(t.childOrder, seg)
	}
	return t.children[seg]
}

func (t *pathTrie) toCollection() Collection {
	var kids []Collection
	for _, k := range t.childOrder {
		kids = append(kids, t.children[k].toCollection())
	}
	return Collection{
		ID:       NewID(),
		Name:     t.name,
		Requests: t.requests,
		Children: kids,
	}
}

// ── Public import function ────────────────────────────────────────────────────

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
		return nil, fmt.Errorf("no paths found — is this a valid OpenAPI/Swagger document?")
	}

	title := doc.Info.Title
	if title == "" {
		title = "Imported API"
	}
	baseURL := resolveBaseURL(doc)

	// Build schema registry for $ref resolution
	schemas := map[string]*openapiSchema{}
	for k, v := range doc.Components.Schemas {
		s := v
		schemas["#/components/schemas/"+k] = s
	}
	for k, v := range doc.Definitions {
		s := v
		schemas["#/definitions/"+k] = &s
	}

	// Root trie named after the API title
	root := newTrie(title)

	httpMethods := []string{"get", "post", "put", "patch", "delete", "head", "options"}
	sortedPaths := sortedKeys(doc.Paths)

	for _, path := range sortedPaths {
		pathItem := doc.Paths[path]
		segs := pathSegments(path)

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

			// Navigate / create trie nodes for each path segment
			node := root
			for _, seg := range segs {
				node = node.child(seg)
			}
			node.requests = append(node.requests, req)
		}
	}

	// Convert root's direct children to top-level collections
	var cols []Collection
	for _, k := range root.childOrder {
		cols = append(cols, root.children[k].toCollection())
	}

	// If nothing grouped (all requests at "/"), put them under the API title
	if len(cols) == 0 && len(root.requests) > 0 {
		cols = []Collection{{
			ID:       NewID(),
			Name:     title,
			Requests: root.requests,
		}}
	}

	return cols, nil
}

// ── Build helpers ─────────────────────────────────────────────────────────────

func buildRequest(method, path, baseURL string, op openapiOperation, schemas map[string]*openapiSchema) Request {
	name := op.OperationID
	if name == "" {
		name = op.Summary
	}
	if name == "" {
		name = strings.ToUpper(method) + " " + path
	}

	url := baseURL + path
	var queryParts []string
	for _, p := range op.Parameters {
		if p.In == "query" {
			queryParts = append(queryParts, p.Name+"=")
		}
	}
	if len(queryParts) > 0 {
		sort.Strings(queryParts)
		url += "?" + strings.Join(queryParts, "&")
	}

	headers := map[string]string{}
	body := ""

	if op.RequestBody != nil {
		if mt, ok := op.RequestBody.Content["application/json"]; ok {
			headers["Content-Type"] = "application/json"
			body = extractBody(mt, schemas)
		} else if _, ok := op.RequestBody.Content["multipart/form-data"]; ok {
			headers["Content-Type"] = "multipart/form-data"
		} else if _, ok := op.RequestBody.Content["application/x-www-form-urlencoded"]; ok {
			headers["Content-Type"] = "application/x-www-form-urlencoded"
		} else {
			for ct, mt := range op.RequestBody.Content {
				headers["Content-Type"] = ct
				body = extractBody(mt, schemas)
				break
			}
		}
	}
	// Swagger 2.x body parameter
	for _, p := range op.Parameters {
		if p.In == "body" && p.Schema != nil {
			headers["Content-Type"] = "application/json"
			ex := generateExample(resolveRef(p.Schema, schemas), schemas, 0)
			if b, err := prettyJSON(ex); err == nil {
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

func extractBody(mt openapiMedia, schemas map[string]*openapiSchema) string {
	if mt.Example != nil && string(mt.Example) != "null" {
		if b, err := prettyJSONRaw(mt.Example); err == nil {
			return b
		}
	}
	if mt.Schema != nil {
		ex := generateExample(resolveRef(mt.Schema, schemas), schemas, 0)
		if b, err := prettyJSON(ex); err == nil {
			return b
		}
	}
	return ""
}

func resolveRef(s *openapiSchema, schemas map[string]*openapiSchema) *openapiSchema {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		if r, ok := schemas[s.Ref]; ok {
			return r
		}
	}
	return s
}

func generateExample(s *openapiSchema, schemas map[string]*openapiSchema, depth int) interface{} {
	if s == nil || depth > 5 {
		return nil
	}
	s = resolveRef(s, schemas)
	if s == nil {
		return nil
	}
	if s.Example != nil && string(s.Example) != "null" {
		var v interface{}
		if json.Unmarshal(s.Example, &v) == nil {
			return v
		}
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	if len(s.AllOf) > 0 {
		merged := map[string]interface{}{}
		for _, sub := range s.AllOf {
			if v, ok := generateExample(resolveRef(sub, schemas), schemas, depth+1).(map[string]interface{}); ok {
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
		for _, k := range sortedKeys(s.Properties) {
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
		default:
			return "string"
		}
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return true
	}
	return nil
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func resolveBaseURL(doc openapiDoc) string {
	if len(doc.Servers) > 0 {
		return strings.TrimRight(doc.Servers[0].URL, "/")
	}
	if doc.Host != "" {
		scheme := "https"
		if len(doc.Schemes) > 0 {
			scheme = doc.Schemes[0]
		}
		return scheme + "://" + doc.Host + doc.BasePath
	}
	return ""
}

func pathSegments(path string) []string {
	var segs []string
	for _, s := range strings.Split(strings.Trim(path, "/"), "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
