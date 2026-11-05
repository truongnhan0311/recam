// Command protocheck verifies the protocol documents (plan.md, section 9, C1).
//
// Run from the repository root:  go run ./tools/protocheck
//
// Checks:
//  1. docs/openapi.yaml is a valid OpenAPI 3.1 document, every $ref resolves,
//     and JSON Schema keywords (type, required, enum) are well-formed.
//  2. Every operation declares at least one error response that uses the
//     shared Problem schema (application/problem+json).
//  3. Every DataChannel (Ctrl*) message schema requires the version field `v`.
//  4. MQTT messages (Mq*): the header requires `v`, every message extends
//     MqEnvelope, and every message named in x-mqtt exists.
//  5. docs/PROTOCOL.md has a mermaid stateDiagram, a mermaid sequenceDiagram
//     and a heading for each of the 10 lifecycle stages.
//
// Exit code 0 only if every check passes.
package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"gopkg.in/yaml.v3"
)

const (
	openapiPath  = "docs/openapi.yaml"
	protocolPath = "docs/PROTOCOL.md"
	schemaPrefix = "#/components/schemas/"
)

var failures []string

func fail(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	failures = append(failures, msg)
	fmt.Println("FAIL:", msg)
}

func ok(format string, a ...any) { fmt.Printf("ok:   "+format+"\n", a...) }

type obj = map[string]any

func asObj(v any) obj {
	if m, isMap := v.(map[string]any); isMap {
		return m
	}
	return obj{}
}

func asList(v any) []any {
	if l, isList := v.([]any); isList {
		return l
	}
	return nil
}

// resolve follows a local "#/a/b/c" reference.
func resolve(spec obj, ref string) obj {
	node := any(spec)
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		node = asObj(node)[part]
	}
	return asObj(node)
}

func requiredOf(spec obj, schema obj) map[string]bool {
	req := map[string]bool{}
	add := func(s obj) {
		for _, r := range asList(s["required"]) {
			if str, isStr := r.(string); isStr {
				req[str] = true
			}
		}
	}
	add(schema)
	for _, sub := range asList(schema["allOf"]) {
		subm := asObj(sub)
		if ref, isRef := subm["$ref"].(string); isRef {
			add(resolve(spec, ref))
		}
		add(subm)
	}
	return req
}

func usesProblem(spec obj, resp obj) bool {
	if ref, isRef := resp["$ref"].(string); isRef {
		resp = resolve(spec, ref)
	}
	media := asObj(asObj(resp["content"])["application/problem+json"])
	return asObj(media["schema"])["$ref"] == schemaPrefix+"Problem"
}

func checkOpenAPIValid(data []byte) bool {
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		fail("openapi.yaml cannot be parsed: %v", err)
		return false
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		for _, e := range errs {
			fail("openapi.yaml validator setup: %v", e)
		}
		return false
	}
	valid, verrs := v.ValidateDocument()
	if !valid {
		for _, e := range verrs {
			fail("openapi.yaml invalid: %s (%s)", e.Message, e.Reason)
		}
		return false
	}
	ok("openapi.yaml is valid (%s)", libopenapiVersion(doc))
	return true
}

func libopenapiVersion(doc libopenapi.Document) string {
	return doc.GetSpecInfo().Version
}

func checkOperations(spec obj) {
	methods := []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	n, before := 0, len(failures)
	paths := asObj(spec["paths"])
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, path := range names {
		item := asObj(paths[path])
		for _, m := range methods {
			opAny, has := item[m]
			if !has {
				continue
			}
			n++
			found := false
			for code, r := range asObj(asObj(opAny)["responses"]) {
				if (code == "default" || strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5")) &&
					usesProblem(spec, asObj(r)) {
					found = true
				}
			}
			if !found {
				fail("%s %s has no Problem error response", strings.ToUpper(m), path)
			}
		}
	}
	if len(failures) == before {
		ok("all %d operations declare Problem error responses", n)
	}
}

func checkMessageSchemas(spec obj) {
	schemas := asObj(asObj(spec["components"])["schemas"])
	before := len(failures)

	var ctrl, mq []string
	for name := range schemas {
		switch {
		case strings.HasPrefix(name, "Ctrl"):
			ctrl = append(ctrl, name)
		case strings.HasPrefix(name, "Mq") &&
			name != "MqHeader" && name != "MqEnvelope" && name != "MqttAccessConfig":
			mq = append(mq, name)
		}
	}
	sort.Strings(ctrl)
	sort.Strings(mq)

	if len(ctrl) == 0 {
		fail("no DataChannel schemas (names starting with Ctrl)")
	}
	for _, n := range ctrl {
		if !requiredOf(spec, asObj(schemas[n]))["v"] {
			fail("schema %s does not require field 'v'", n)
		}
	}

	if !requiredOf(spec, asObj(schemas["MqHeader"]))["v"] {
		fail("MqHeader does not require field 'v'")
	}
	env := requiredOf(spec, asObj(schemas["MqEnvelope"]))
	if !env["header"] || !env["msg"] {
		fail("MqEnvelope must require 'header' and 'msg'")
	}
	for _, n := range mq {
		extends := false
		for _, sub := range asList(asObj(schemas[n])["allOf"]) {
			if asObj(sub)["$ref"] == schemaPrefix+"MqEnvelope" {
				extends = true
			}
		}
		if !extends {
			fail("schema %s does not extend MqEnvelope", n)
		}
	}

	for _, t := range asList(asObj(spec["x-mqtt"])["topics"]) {
		topic := asObj(t)
		for _, m := range asList(topic["messages"]) {
			name, _ := m.(string)
			if _, exists := schemas[name]; !exists {
				fail("x-mqtt topic %v lists unknown message %s", topic["name"], name)
			}
		}
	}

	if len(failures) == before {
		ok("Ctrl* (%d) and Mq* (%d) schemas carry 'v'; x-mqtt messages all defined", len(ctrl), len(mq))
	}
}

var jsonTypes = map[string]bool{
	"string": true, "number": true, "integer": true, "boolean": true,
	"array": true, "object": true, "null": true,
}

// walkSchema checks JSON Schema keywords the OpenAPI validator lets through
// (for example a misspelled "type", or non-string entries in "required").
func walkSchema(where string, node any) {
	s, isMap := node.(map[string]any)
	if !isMap {
		return // boolean schemas and absent nodes are fine
	}
	if t, has := s["type"]; has {
		switch tv := t.(type) {
		case string:
			if !jsonTypes[tv] {
				fail("%s: unknown schema type %q", where, tv)
			}
		case []any:
			for _, e := range tv {
				if str, isStr := e.(string); !isStr || !jsonTypes[str] {
					fail("%s: invalid entry %v in schema type list", where, e)
				}
			}
		default:
			fail("%s: schema type must be a string or list, got %T", where, t)
		}
	}
	if r, has := s["required"]; has {
		list, isList := r.([]any)
		if !isList {
			fail("%s: required must be a list", where)
		}
		for _, e := range list {
			if _, isStr := e.(string); !isStr {
				fail("%s: required contains non-string %v (%T)", where, e, e)
			}
		}
	}
	if e, has := s["enum"]; has {
		if _, isList := e.([]any); !isList {
			fail("%s: enum must be a list", where)
		}
	}
	for name, sub := range asObj(s["properties"]) {
		walkSchema(where+".properties."+name, sub)
	}
	walkSchema(where+".items", s["items"])
	walkSchema(where+".additionalProperties", s["additionalProperties"])
	walkSchema(where+".not", s["not"])
	for _, kw := range []string{"allOf", "oneOf", "anyOf"} {
		for i, sub := range asList(s[kw]) {
			walkSchema(fmt.Sprintf("%s.%s[%d]", where, kw, i), sub)
		}
	}
}

func walkContent(where string, content any) {
	for media, m := range asObj(content) {
		walkSchema(where+"."+media, asObj(m)["schema"])
	}
}

func checkSchemaKeywords(spec obj) {
	before := len(failures)
	comps := asObj(spec["components"])
	for name, sc := range asObj(comps["schemas"]) {
		walkSchema("components.schemas."+name, sc)
	}
	for name, prm := range asObj(comps["parameters"]) {
		walkSchema("components.parameters."+name, asObj(prm)["schema"])
	}
	for name, r := range asObj(comps["responses"]) {
		walkContent("components.responses."+name, asObj(r)["content"])
		for h, hv := range asObj(asObj(r)["headers"]) {
			walkSchema("components.responses."+name+".headers."+h, asObj(hv)["schema"])
		}
	}
	for path, item := range asObj(spec["paths"]) {
		for method, opAny := range asObj(item) {
			op, isOp := opAny.(map[string]any)
			if !isOp {
				continue
			}
			where := method + " " + path
			for i, prm := range asList(op["parameters"]) {
				walkSchema(fmt.Sprintf("%s.parameters[%d]", where, i), asObj(prm)["schema"])
			}
			walkContent(where+".requestBody", asObj(op["requestBody"])["content"])
			for code, r := range asObj(op["responses"]) {
				walkContent(where+".responses."+code, asObj(r)["content"])
			}
		}
	}
	if len(failures) == before {
		ok("schema keywords (type, required, enum) are well-formed")
	}
}

// walkRefs verifies that every "$ref" points to something that exists.
func walkRefs(spec obj, where string, node any, bad *int) {
	switch n := node.(type) {
	case map[string]any:
		if ref, isStr := n["$ref"].(string); isStr && strings.HasPrefix(ref, "#/") {
			cur := any(spec)
			for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				next, has := asObj(cur)[part]
				if !has {
					fail("%s: $ref %s does not resolve", where, ref)
					*bad++
					cur = nil
					break
				}
				cur = next
			}
		}
		for k, v := range n {
			walkRefs(spec, where+"."+k, v, bad)
		}
	case []any:
		for i, v := range n {
			walkRefs(spec, fmt.Sprintf("%s[%d]", where, i), v, bad)
		}
	}
}

func checkRefs(spec obj) {
	bad := 0
	walkRefs(spec, "spec", spec, &bad)
	if bad == 0 {
		ok("every $ref resolves")
	}
}

func checkProtocolDoc(md string) {
	before := len(failures)
	if !regexp.MustCompile("(?s)```mermaid\\s+stateDiagram").MatchString(md) {
		fail("PROTOCOL.md has no mermaid stateDiagram")
	}
	if !regexp.MustCompile("(?s)```mermaid\\s+sequenceDiagram").MatchString(md) {
		fail("PROTOCOL.md has no mermaid sequenceDiagram")
	}
	for i := 1; i <= 10; i++ {
		re := regexp.MustCompile(fmt.Sprintf(`(?m)^#{2,4} Giai đoạn %d\b`, i))
		if !re.MatchString(md) {
			fail("PROTOCOL.md missing heading 'Giai đoạn %d'", i)
		}
	}
	if len(failures) == before {
		ok("PROTOCOL.md has state diagram, sequence diagram, and 10 stages")
	}
}

func main() {
	data, err := os.ReadFile(openapiPath)
	if err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}
	md, err := os.ReadFile(protocolPath)
	if err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}

	var spec obj
	if err := yaml.Unmarshal(data, &spec); err != nil {
		fmt.Println("FAIL: openapi.yaml is not valid YAML:", err)
		os.Exit(1)
	}

	if checkOpenAPIValid(data) {
		checkRefs(spec)
		checkSchemaKeywords(spec)
		checkOperations(spec)
		checkMessageSchemas(spec)
	}
	checkProtocolDoc(string(md))

	if len(failures) > 0 {
		fmt.Printf("\n%d check(s) failed\n", len(failures))
		os.Exit(1)
	}
	fmt.Println("\nC1: all checks passed")
}
