// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package router

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/pb33f/libopenapi/orderedmap"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type compiledServer struct {
	expression *regexp.Regexp
	names      []string
	absolute   bool
}

func compileDocumentServers(servers []*v3.Server) map[*v3.Server]*compiledServer {
	compiled := make(map[*v3.Server]*compiledServer, len(servers))
	for _, server := range servers {
		template, absolute := normalizeServerTemplate(server)
		expression, names, err := compileServerTemplate(template)
		if err == nil {
			compiled[server] = &compiledServer{expression: expression, names: names, absolute: absolute}
		}
	}
	return compiled
}

func collectServers(document *v3.Document) []*v3.Server {
	if document == nil {
		return nil
	}
	seen := make(map[*v3.Server]struct{})
	var servers []*v3.Server
	add := func(values []*v3.Server) {
		for _, server := range values {
			if server == nil {
				continue
			}
			if _, ok := seen[server]; !ok {
				seen[server] = struct{}{}
				servers = append(servers, server)
			}
		}
	}
	add(document.Servers)
	if document.Paths != nil && document.Paths.PathItems != nil {
		for pair := orderedmap.First(document.Paths.PathItems); pair != nil; pair = pair.Next() {
			item := pair.Value()
			add(item.Servers)
			for operation := item.GetOperations().First(); operation != nil; operation = operation.Next() {
				add(operation.Value().Servers)
			}
		}
	}
	return servers
}

func serverIsEffective(server *v3.Server, operation *v3.Operation, item *v3.PathItem, document *v3.Document) bool {
	var effective []*v3.Server
	if operation != nil && len(operation.Servers) > 0 {
		effective = operation.Servers
	} else if item != nil && len(item.Servers) > 0 {
		effective = item.Servers
	} else if document != nil {
		effective = document.Servers
	}
	for _, candidate := range effective {
		if candidate == server {
			return true
		}
	}
	return false
}

func matchServer(request *http.Request, server *v3.Server) (string, map[string]string, bool) {
	if request == nil || request.URL == nil || server == nil {
		return "", nil, false
	}
	template, absolute := normalizeServerTemplate(server)
	expression, names, err := compileServerTemplate(template)
	if err != nil {
		return "", nil, false
	}
	return matchCompiledServer(request, server, &compiledServer{expression: expression, names: names, absolute: absolute})
}

func normalizeServerTemplate(server *v3.Server) (string, bool) {
	template := server.URL
	if template == "" {
		template = "/"
	}
	absolute := strings.Contains(template, "://")
	if !absolute && !strings.HasPrefix(template, "/") {
		template = "/" + template
	}
	template = strings.TrimSuffix(template, "/")
	if template == "" {
		template = "/"
	}
	return template, absolute
}

func matchCompiledServer(request *http.Request, server *v3.Server, compiled *compiledServer) (string, map[string]string, bool) {
	if request == nil || request.URL == nil || server == nil || compiled == nil {
		return "", nil, false
	}
	target := request.URL.EscapedPath()
	if compiled.absolute {
		scheme := request.URL.Scheme
		if scheme == "" {
			if request.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}
		host := request.URL.Host
		if host == "" {
			host = request.Host
		}
		target = scheme + "://" + host + target
	}

	matches := compiled.expression.FindStringSubmatch(target)
	if matches == nil {
		return "", nil, false
	}
	params := make(map[string]string, len(compiled.names))
	for i, name := range compiled.names {
		value := decodePathValue(matches[i+1])
		if variable := serverVariable(server, name); variable != nil && len(variable.Enum) > 0 && !contains(variable.Enum, value) {
			return "", nil, false
		}
		params[name] = value
	}
	rest := matches[len(matches)-1]
	if rest == "" {
		rest = "/"
	}
	return rest, params, true
}

func compileServerTemplate(template string) (*regexp.Regexp, []string, error) {
	if template == "/" {
		expression, err := regexp.Compile(`^(/.*|/)$`)
		return expression, nil, err
	}
	indices, err := braceIndices(template)
	if err != nil {
		return nil, nil, err
	}
	var pattern strings.Builder
	pattern.WriteByte('^')
	names := make([]string, 0, len(indices)/2)
	end := 0
	for i := 0; i < len(indices); i += 2 {
		pattern.WriteString(regexp.QuoteMeta(template[end:indices[i]]))
		end = indices[i+1]
		name := template[indices[i]+1 : end-1]
		if name == "" {
			return nil, nil, &url.Error{Op: "parse", URL: template, Err: url.InvalidHostError("empty server variable")}
		}
		names = append(names, name)
		pattern.WriteString("([^/?#]+)")
	}
	pattern.WriteString(regexp.QuoteMeta(template[end:]))
	pattern.WriteString("(/.*|$)")
	expression, err := regexp.Compile(pattern.String())
	return expression, names, err
}

func serverVariable(server *v3.Server, name string) *v3.ServerVariable {
	if server == nil || server.Variables == nil {
		return nil
	}
	return server.Variables.GetOrZero(name)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func compatibilityPath(request *http.Request, document *v3.Document) string {
	path := request.URL.EscapedPath()
	if document != nil {
		for _, server := range document.Servers {
			if server == nil {
				continue
			}
			parsed, err := url.Parse(server.URL)
			if err == nil && parsed.Path != "" && strings.HasPrefix(path, parsed.Path) {
				path = strings.TrimPrefix(path, parsed.Path)
				break
			}
		}
	}
	if request.URL.Fragment != "" {
		path += "#" + request.URL.Fragment
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func compatibilityServer(request *http.Request, document *v3.Document) *v3.Server {
	if request == nil || request.URL == nil || document == nil {
		return nil
	}
	path := request.URL.EscapedPath()
	for _, server := range document.Servers {
		if server == nil {
			continue
		}
		parsed, err := url.Parse(server.URL)
		if err == nil && parsed.Path != "" && strings.HasPrefix(path, parsed.Path) {
			return server
		}
	}
	return nil
}
