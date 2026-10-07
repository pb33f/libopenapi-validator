// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/pb33f/go-yaml"
	upstream "github.com/pb33f/libopenapi/arazzo"
)

// sourceSession holds only request-local source metadata. SourceDescription names
// belong to their containing document; canonical identities belong to documents.
type sourceSession struct {
	v                   *validation
	base                string
	byURI               map[string]*linkedSource
	byName              map[string]map[string]any
	requests            map[string]*linkedSource
	count, bytes, nodes int
	graphVisited        map[*linkedSource]bool
	scopePath           string
	scopeSource         *linkedSource
	descriptionPaths    map[string]string
	targets             map[dependencyID]*linkedTarget
}

type linkedSource struct {
	owner                           *sourceSession
	nodes                           map[string]nodeLocation
	unresolvedRefs                  bool
	partialExpressions              bool
	refMemo                         map[string]map[string]any
	kind, identity, retrieval, base string
	root                            map[string]any
	adapter                         upstream.SourceDocumentAdapter
	operations                      map[string][]*linkedTarget
	pointers                        map[string]*linkedTarget
	workflows                       map[string]map[string]any
	steps                           map[string]map[string]map[string]any
}

type linkedTarget struct {
	source             *linkedSource
	kind               string
	node               map[string]any
	parameters         map[linkedParameterKey]bool
	parametersComplete bool
}
type linkedParameterKey struct{ name, in string }

func checkSources(v *validation) {
	s := &sourceSession{v: v, byURI: make(map[string]*linkedSource), byName: make(map[string]map[string]any), requests: make(map[string]*linkedSource), graphVisited: make(map[*linkedSource]bool)}
	v.sources = s
	s.descriptionPaths = make(map[string]string)
	s.targets = make(map[dependencyID]*linkedTarget)
	s.bytes = v.byteCount
	s.nodes = v.nodeCount
	s.base = resolveLinkedURI(v.doc.URI, v.opts.baseURI)
	if self := text(v.root["$self"]); self != "" {
		s.base = resolveLinkedURI(self, s.base)
	}
	for i, value := range array(v.root["sourceDescriptions"]) {
		description := object(value)
		name := text(description["name"])
		if _, exists := s.byName[name]; !exists {
			s.byName[name] = description
			s.descriptionPaths[name] = fmt.Sprintf("/sourceDescriptions/%d", i)
		}
	}
	// Fully index every candidate before looking up any reference. A later supplied
	// candidate can establish the $self identity of an earlier reference.
	primary := &linkedSource{owner: s, kind: "arazzo", identity: v.documentID(), retrieval: v.doc.URI, base: s.base, root: v.root, workflows: make(map[string]map[string]any)}
	primary.indexRaw()
	if s.base != "" {
		s.byURI[s.base] = primary
	}
	if v.doc.URI != "" {
		s.byURI[v.doc.URI] = primary
	}
	s.graphVisited[primary] = true
	for _, candidate := range v.opts.sources {
		if !v.check() {
			return
		}
		source := &upstream.ResolvedSource{Type: candidate.Type, Identity: candidate.ResolvedIdentity, RetrievalURI: candidate.RetrievalURI, SourceBytes: candidate.SourceBytes, RootNode: candidate.RootNode, OpenAPIDocument: candidate.OpenAPIDocument, ArazzoDocument: candidate.ArazzoDocument, Adapter: candidate.Adapter}
		item := s.build(source)
		if v.err != nil {
			return
		}
		if item != nil && !s.register(item) {
			return
		}
	}
	for i, value := range array(v.root["sourceDescriptions"]) {
		description := object(value)
		uri := resolveLinkedURI(text(description["url"]), s.base)
		if source := s.byURI[uri]; source != nil {
			s.checkDescription(source, description, fmt.Sprintf("/sourceDescriptions/%d", i))
		}
	}
	for wi, value := range array(v.root["workflows"]) {
		if !v.check() {
			return
		}
		workflow := object(value)
		path := fmt.Sprintf("/workflows/%d", wi)
		for di, dep := range array(workflow["dependsOn"]) {
			if strings.HasPrefix(text(dep), "$sourceDescriptions.") {
				depPath := fmt.Sprintf("%s/dependsOn/%d", path, di)
				target := s.workflow(text(dep), depPath)
				if target != nil {
					_, id, _ := s.qualified(text(dep))
					to := dependencyID{document: target.source.identity, workflow: id}
					from := dependencyID{document: v.documentID(), workflow: text(workflow["workflowId"]), prerequisites: true}
					v.graph.addEdge(from, to, depPath)
					s.graphSource(target.source, depPath, 1)
				}
			}
		}
		s.actions(workflow["successActions"], path+"/successActions")
		s.actions(workflow["failureActions"], path+"/failureActions")
		for si, value := range array(workflow["steps"]) {
			step := object(value)
			stepPath := fmt.Sprintf("%s/steps/%d", path, si)
			target := s.step(step, stepPath)
			s.targets[dependencyID{document: v.documentID(), workflow: text(workflow["workflowId"]), step: text(step["stepId"])}] = target
			if target != nil {
				s.parameters(workflow["parameters"], path+"/parameters", step["parameters"], stepPath+"/parameters", target)
			}
			s.actions(step["onSuccess"], stepPath+"/onSuccess")
			s.actions(step["onFailure"], stepPath+"/onFailure")
			for di, dep := range array(step["dependsOn"]) {
				if strings.HasPrefix(text(dep), "$sourceDescriptions.") {
					depPath := fmt.Sprintf("%s/dependsOn/%d", stepPath, di)
					if target, ok := s.externalStep(text(dep), depPath); ok {
						v.graph.addEdge(dependencyID{document: v.documentID(), workflow: text(workflow["workflowId"]), step: text(step["stepId"])}, target, depPath)
					}
				}
			}
		}
	}
	s.requestExpressions(v.expressionUses)
	for _, collection := range []string{"successActions", "failureActions"} {
		definitions := object(object(v.root["components"])[collection])
		for _, name := range semanticKeys(definitions) {
			value := definitions[name]
			action := object(value)
			path := joinPtr("/components/"+collection, name)
			if strings.HasPrefix(text(action["workflowId"]), "$sourceDescriptions.") {
				s.workflow(text(action["workflowId"]), path+"/workflowId")
			}
		}
	}
}

func resolveLinkedURI(ref, base string) string {
	uri, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if base != "" {
		b, e := url.Parse(base)
		if e == nil {
			uri = b.ResolveReference(uri)
		}
	}
	if uri.Scheme != "" {
		uri.Scheme = strings.ToLower(uri.Scheme)
		uri.Host = strings.ToLower(uri.Host)
	}
	return uri.String()
}

func (s *sourceSession) fail(kind ErrorKind, message string) {
	if s.v.err == nil {
		s.v.err = &Error{Kind: kind, Cause: errors.New(message)}
	}
}

func (s *sourceSession) build(source *upstream.ResolvedSource) *linkedSource {
	if source == nil {
		return nil
	}
	s.count++
	if s.count > s.v.opts.limits.MaxSources {
		s.fail(ErrorLimit, "total source document limit exceeded")
		return nil
	}
	s.bytes += len(source.SourceBytes)
	if s.bytes > s.v.opts.limits.MaxBytes {
		s.fail(ErrorLimit, "total source byte limit exceeded")
		return nil
	}
	item := &linkedSource{owner: s, adapter: source.Adapter, operations: make(map[string][]*linkedTarget), pointers: make(map[string]*linkedTarget), workflows: make(map[string]map[string]any)}
	retrieval := source.RetrievalURI
	if retrieval == "" {
		retrieval = source.URL
	}
	item.retrieval = resolveLinkedURI(retrieval, s.base)
	root := source.RootNode
	if root == nil && source.ArazzoDocument != nil && source.ArazzoDocument.GoLow() != nil {
		root = source.ArazzoDocument.GoLow().RootNode
	}
	if root == nil && source.OpenAPIDocument != nil && source.OpenAPIDocument.GoLow() != nil && source.OpenAPIDocument.GoLow().Index != nil {
		root = source.OpenAPIDocument.GoLow().Index.GetRootNode()
	}
	if root == nil && len(source.SourceBytes) > 0 {
		var parsed yaml.Node
		decoder := yaml.NewDecoder(bytes.NewReader(source.SourceBytes))
		if err := decoder.Decode(&parsed); err != nil {
			s.fail(ErrorInput, fmt.Sprintf("parse source %q: %v", item.retrieval, err))
			return nil
		}
		var extra yaml.Node
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			s.fail(ErrorInput, "source contains multiple YAML documents")
			return nil
		}
		root = &parsed
	}
	if root != nil {
		limits := s.v.opts.limits
		limits.MaxBytes -= s.bytes
		limits.MaxNodes -= s.nodes
		if limits.MaxBytes <= 0 || limits.MaxNodes <= 0 {
			s.fail(ErrorLimit, "total source document resources exceeded")
			return nil
		}
		value, nodes, diagnostics, stats, err := normalizeWithStats(s.v.ctx, Document{Root: root, URI: item.retrieval}, limits)
		if err != nil {
			s.v.err = err
			return nil
		}
		if len(diagnostics) > 0 {
			s.fail(ErrorInput, fmt.Sprintf("source %q is not a JSON-compatible document", item.retrieval))
			return nil
		}
		s.nodes += stats.nodes
		s.bytes += stats.bytes
		if s.nodes > s.v.opts.limits.MaxNodes {
			s.fail(ErrorLimit, "total source node limit exceeded")
			return nil
		}
		item.root = object(value)
		item.nodes = nodes
		switch {
		case item.root["openapi"] != nil:
			item.kind = "openapi"
		case item.root["arazzo"] != nil:
			item.kind = "arazzo"
		case item.root["asyncapi"] != nil:
			item.kind = "asyncapi"
		}
	}
	if item.kind == "" {
		switch {
		case source.OpenAPIDocument != nil:
			item.kind = "openapi"
		case source.ArazzoDocument != nil:
			item.kind = "arazzo"
		case source.Adapter != nil:
			item.kind = strings.ToLower(source.Adapter.SourceType())
		}
	}
	if item.kind == "" {
		s.v.incomplete("source-kind", "", item.retrieval, "resolver supplied no parsed document or typed adapter")
		item.identity = item.retrieval
		return item
	}
	item.identity = resolveLinkedURI(source.Identity, item.retrieval)
	if item.identity == "" {
		item.identity = item.retrieval
	}
	self := text(item.root["$self"])
	if self == "" && source.ArazzoDocument != nil {
		self = source.ArazzoDocument.Self
	}
	if item.kind == "arazzo" && self != "" {
		canonical := resolveLinkedURI(self, item.retrieval)
		if source.Identity != "" && item.identity != canonical {
			s.fail(ErrorConfiguration, "supplied source identity conflicts with its $self URI")
			return nil
		}
		item.identity = canonical
	}
	item.base = item.identity
	if item.base == "" {
		item.base = item.retrieval
	}
	if item.root != nil {
		item.indexRaw()
	} else if source.OpenAPIDocument != nil {
		item.indexOpenAPI(source.OpenAPIDocument, s)
	} else if source.ArazzoDocument != nil {
		item.partialExpressions = true
		item.root = map[string]any{"arazzo": source.ArazzoDocument.Arazzo, "$self": source.ArazzoDocument.Self}
		descriptions := []any{}
		for _, description := range source.ArazzoDocument.SourceDescriptions {
			if description != nil {
				if !s.charge(7, len(description.Name)+len(description.URL)+len(description.Type)) {
					return nil
				}
				descriptions = append(descriptions, map[string]any{"name": description.Name, "url": description.URL, "type": description.Type})
			}
		}
		item.root["sourceDescriptions"] = descriptions
		workflows := []any{}
		for _, workflow := range source.ArazzoDocument.Workflows {
			if workflow == nil {
				continue
			}
			if !s.charge(6, len(workflow.WorkflowId)) {
				return nil
			}
			wf := map[string]any{"workflowId": workflow.WorkflowId, "dependsOn": s.linkedStrings(workflow.DependsOn), "outputs": linkedOutputs(workflow.Outputs, s)}
			if workflow.Inputs != nil {
				limits := s.v.opts.limits
				limits.MaxBytes -= s.bytes
				limits.MaxNodes -= s.nodes
				if limits.MaxBytes <= 0 || limits.MaxNodes <= 0 {
					s.fail(ErrorLimit, "total source document resources exceeded")
					return nil
				}
				value, _, diags, stats, err := normalizeWithStats(s.v.ctx, Document{Root: workflow.Inputs, URI: item.retrieval}, limits)
				if err != nil {
					s.v.err = err
					return nil
				}
				if len(diags) > 0 {
					s.fail(ErrorInput, "supplied workflow input schema is not JSON-compatible")
					return nil
				}
				if !s.charge(stats.nodes, stats.bytes) {
					return nil
				}
				wf["inputs"] = value
			}
			steps := []any{}
			for _, step := range workflow.Steps {
				if step != nil {
					if !s.charge(6, len(step.StepId)) {
						return nil
					}
					steps = append(steps, map[string]any{"stepId": step.StepId, "dependsOn": s.linkedStrings(step.DependsOn), "outputs": linkedOutputs(step.Outputs, s)})
				}
			}
			wf["steps"] = steps
			if s.v.err != nil {
				return nil
			}
			workflows = append(workflows, wf)
			item.workflows[workflow.WorkflowId] = wf
		}
		item.root["workflows"] = workflows
		item.indexSteps()
	}

	return item
}

func (s *sourceSession) register(item *linkedSource) bool {
	for _, uri := range []string{item.identity, item.retrieval} {
		if uri == "" {
			continue
		}
		if other := s.byURI[uri]; other != nil && other != item {
			s.fail(ErrorConfiguration, fmt.Sprintf("duplicate source document identity %q", uri))
			return false
		}
	}
	for _, uri := range []string{item.identity, item.retrieval} {
		if uri != "" {
			s.byURI[uri] = item
		}
	}
	return true
}

func (s *sourceSession) checkDescription(source *linkedSource, description map[string]any, path string) bool {
	expected := text(description["type"])
	if source.kind != "" {
		s.complete("source-kind", s.descriptionFieldPath(path, "type"), source.identity)
	}
	if expected != "" && source.kind != "" && expected != source.kind {
		s.v.add(CodeSourceType, s.descriptionFieldPath(path, "type"), fmt.Sprintf("source declares %s but supplied document is %s", expected, source.kind))
		return false
	}
	uri := resolveLinkedURI(text(description["url"]), s.base)
	if source.kind == "arazzo" && source.identity != "" && source.identity != source.retrieval && uri != source.identity {
		s.v.add(CodeReference, s.descriptionFieldPath(path, "url"), "Arazzo source reference must use the target $self identity")
		return false
	}
	return true
}

func (s *sourceSession) descriptionPath(name string) string {
	if path := s.descriptionPaths[name]; path != "" {
		return path
	}
	if s.scopePath != "" {
		return s.scopePath
	}
	return "/sourceDescriptions"
}

// lookup is also used by dependency validation. It loads a complete document at
// most once per URI and never expands its imports while a resolver is loading it.
func (s *sourceSession) lookup(name, path string) *linkedSource {
	description := s.byName[name]
	if description == nil {
		s.v.add(CodeReference, path, fmt.Sprintf("unknown source description %q", name))
		return nil
	}
	uri := resolveLinkedURI(text(description["url"]), s.base)
	if item, seen := s.requests[uri]; seen {
		if item == nil {
			s.v.incomplete("linked-target", path, name, "source document is unavailable")
		} else if !s.checkDescription(item, description, s.descriptionPath(name)) {
			return nil
		}
		return item
	}
	item := s.byURI[uri]
	if item == nil && s.v.opts.resolver != nil && s.v.check() {
		if s.count >= s.v.opts.limits.MaxSources {
			s.fail(ErrorLimit, "total source document limit exceeded")
			return nil
		}
		resolved, err := s.v.opts.resolver.Resolve(s.v.ctx, upstream.SourceRequest{Name: name, URL: uri, Type: text(description["type"]), BaseURI: ""})
		if err != nil {
			if errors.Is(err, upstream.ErrUnresolvedSourceDesc) {
				s.requests[uri] = nil
				s.v.incomplete("linked-target", path, name, "source document is unavailable")
				return nil
			}
			s.v.err = &Error{Kind: ErrorOperational, Location: s.v.location(path), Cause: err}
			return nil
		}
		item = s.build(resolved)
		if s.v.err != nil {
			return nil
		}
		if item != nil && !s.register(item) {
			return nil
		}
	}
	s.requests[uri] = item
	if item == nil {
		s.v.incomplete("linked-target", path, name, "source document was not supplied")
		return nil
	}
	if !s.checkDescription(item, description, s.descriptionPath(name)) {
		return nil
	}
	if item.kind == "" {
		s.v.incomplete("linked-target", path, name, "resolver returned an incomplete source placeholder")
		return nil
	}
	return item
}

func sourceQualified(value string) (name, target string, ok bool) {
	if !strings.HasPrefix(value, "$sourceDescriptions.") {
		return "", "", false
	}
	rest := strings.TrimPrefix(value, "$sourceDescriptions.")
	index := strings.IndexByte(rest, '.')
	if index < 1 || index == len(rest)-1 {
		return "", "", false
	}
	return rest[:index], rest[index+1:], true
}

func (s *sourceSession) qualified(value string) (name, target string, ok bool) {
	if !strings.HasPrefix(value, "$sourceDescriptions.") {
		return "", "", false
	}
	rest := strings.TrimPrefix(value, "$sourceDescriptions.")
	for candidate := range s.byName {
		prefix := candidate + "."
		if len(candidate) > len(name) && strings.HasPrefix(rest, prefix) && len(rest) > len(prefix) {
			name = candidate
			target = strings.TrimPrefix(rest, prefix)
			ok = true
		}
	}
	if ok {
		return
	}
	return sourceQualified(value)
}

func (s *sourceSession) sourcePath(source *linkedSource, pointer string) string {
	uri := source.retrieval
	if uri == "" {
		uri = source.identity
	}
	key := uri + "#" + pointer
	location := Location{URI: uri, Pointer: pointer}
	for ancestor := pointer; ; {
		if node := source.nodes[ancestor].value; node != nil {
			location.Line, location.Column = node.Line, node.Column
			break
		}
		if ancestor == "" {
			break
		}
		if index := strings.LastIndex(ancestor, "/"); index >= 0 {
			ancestor = ancestor[:index]
		} else {
			ancestor = ""
		}
	}
	if s.v.foreignLocations == nil {
		s.v.foreignLocations = make(map[string]Location)
	}
	s.v.foreignLocations[key] = location
	return key
}

func (s *sourceSession) descriptionFieldPath(path, field string) string {
	if s.scopeSource != nil {
		if location, ok := s.v.foreignLocations[path]; ok {
			return s.sourcePath(s.scopeSource, joinPtr(location.Pointer, field))
		}
	}
	return joinPtr(path, field)
}

func (s *sourceSession) complete(name, path, source string) {
	location := s.v.location(path)
	s.v.recordCheck(Check{Name: name, Status: CheckComplete, URI: location.URI, Pointer: location.Pointer, Source: source})
}

func (s *sourceSession) charge(nodes, bytes int) bool {
	if !s.v.check() {
		return false
	}
	s.nodes += nodes
	s.bytes += bytes
	if s.nodes > s.v.opts.limits.MaxNodes || s.bytes > s.v.opts.limits.MaxBytes {
		s.fail(ErrorLimit, "total source document resources exceeded")
		return false
	}
	return true
}
