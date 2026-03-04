package spec

import (
	"sort"
	"strings"

	"github.com/VladGavrila/gocli-gen/pkg/naming"
)

// groupEndpoints groups parsed endpoints into resources.
//
// Algorithm:
//  1. For each unique path segment under a scope param, count how many distinct
//     CRUD-style endpoints it has.
//  2. Segments with 2+ endpoints become primary resources.
//  3. Segments with only 1 endpoint collapse as actions on the scope's resource.
//  4. Global paths (no scope param) follow standard CRUD detection.
func groupEndpoints(endpoints []*Endpoint) map[string]*Resource {
	// Phase 1: count endpoints per path segment to decide resource vs action
	segCounts := countSegmentEndpoints(endpoints)

	// Phase 2: assign each endpoint to a resource
	resources := make(map[string]*Resource)

	for _, ep := range endpoints {
		segments := splitPath(ep.Path)
		resName, action := assignEndpoint(ep, segments, segCounts)
		if resName == "" {
			continue
		}

		res, ok := resources[resName]
		if !ok {
			res = &Resource{
				Name:     resName,
				Plural:   naming.ToPlural(resName),
				GoName:   naming.ToGoName(resName),
				GoPlural: naming.ToGoName(naming.ToPlural(resName)),
				CLIName:  resName,
				Scope:    "global",
			}
			resources[resName] = res
		}

		res.Endpoints = append(res.Endpoints, ep)

		if action != nil {
			if action.ScopeParam != "" && res.Scope == "global" {
				res.Scope = action.ScopeParam
			}
			if !hasAction(res.Actions, action.Name) {
				res.Actions = append(res.Actions, action)
			}
		}
	}

	return resources
}

// segKey uniquely identifies a path segment in context of its scope.
type segKey struct {
	scope   string // scope param name (e.g., "project"), or "" for global
	segment string // the path segment name
}

// countSegmentEndpoints counts how many distinct HTTP operations each path segment has.
func countSegmentEndpoints(endpoints []*Endpoint) map[segKey]int {
	counts := make(map[segKey]int)

	for _, ep := range endpoints {
		segments := splitPath(ep.Path)
		_, _, resSeg := findResourceSegment(segments)
		if resSeg == "" {
			continue
		}

		// Find scope
		var scope string
		for i, seg := range segments {
			if isParam(seg) && i == 0 {
				scope = stripBraces(seg)
				break
			}
		}

		// Check if this endpoint is directly on the resource (not a sub-path)
		_, resIdx, _ := findResourceSegment(segments)
		afterRes := segments[resIdx+1:]

		// Only count endpoints directly on the resource or its {id}
		isDirectEndpoint := true
		for _, s := range afterRes {
			if !isParam(s) {
				isDirectEndpoint = false
				break
			}
		}

		if isDirectEndpoint {
			key := segKey{scope: scope, segment: resSeg}
			counts[key]++
		}
	}

	return counts
}

// assignEndpoint determines which resource an endpoint belongs to.
func assignEndpoint(ep *Endpoint, segments []string, segCounts map[segKey]int) (string, *Action) {
	if len(segments) == 0 {
		return "", nil
	}

	scopeIdx, resIdx, resSeg := findResourceSegment(segments)
	if resSeg == "" {
		return "", nil
	}

	// Determine scope
	var scopeParam string
	if scopeIdx >= 0 && scopeIdx < resIdx {
		scopeParam = stripBraces(segments[scopeIdx])
	}

	// Determine what's after the resource
	afterRes := segments[resIdx+1:]
	var idParam string
	var subSegments []string
	for _, s := range afterRes {
		if isParam(s) && idParam == "" {
			idParam = stripBraces(s)
		} else if !isParam(s) {
			subSegments = append(subSegments, s)
		}
	}

	// How many direct endpoints does this segment have?
	key := segKey{scope: scopeParam, segment: resSeg}
	directCount := segCounts[key]

	// Decision: if this segment has <2 direct endpoints AND has a scope param,
	// collapse it as an action on the scope param's resource.
	if directCount < 2 && scopeParam != "" {
		parentRes := scopeParam
		actionName := resSeg
		if len(subSegments) > 0 {
			actionName = resSeg + "_" + strings.Join(subSegments, "_")
		}
		return parentRes, buildAction(ep, actionName, idParam, scopeParam, subSegments)
	}

	// This is a primary resource — determine its action
	actionName := inferAction(ep.Method, idParam != "", lastNonEmpty(subSegments), ep.OperationID)
	if len(subSegments) > 0 {
		actionName = naming.ToSnakeCase(subSegments[len(subSegments)-1])
	}

	return resSeg, buildAction(ep, actionName, idParam, scopeParam, subSegments)
}

// findResourceSegment finds the first non-parameter segment.
func findResourceSegment(segments []string) (scopeIdx, resIdx int, resSeg string) {
	scopeIdx = -1
	for i, seg := range segments {
		if isParam(seg) {
			if scopeIdx < 0 && resSeg == "" {
				scopeIdx = i
			}
			continue
		}
		resSeg = seg
		resIdx = i
		return scopeIdx, resIdx, resSeg
	}
	return scopeIdx, resIdx, resSeg
}

// buildAction creates an Action from an endpoint and its classification.
func buildAction(ep *Endpoint, actionName, idParam, scopeParam string, subSegments []string) *Action {
	act := &Action{
		Name:        actionName,
		GoName:      naming.ToGoName(actionName),
		CLIName:     naming.ToCLIName(actionName),
		Short:       ep.Summary,
		Method:      ep.Method,
		Path:        ep.Path,
		IDParam:     idParam,
		ScopeParam:  scopeParam,
		HasBody:     ep.RequestBody != nil,
		RequestBody: ep.RequestBody,
		Response:    ep.Response,
	}

	for _, p := range ep.Parameters {
		switch p.In {
		case "path":
			act.PathParams = append(act.PathParams, p)
		case "query":
			act.QueryParams = append(act.QueryParams, p)
		}
	}

	return act
}

// inferAction determines the CRUD action name from HTTP method and path shape.
func inferAction(method string, hasID bool, subAction, operationID string) string {
	if subAction != "" {
		return naming.ToSnakeCase(subAction)
	}

	method = strings.ToUpper(method)
	switch {
	case method == "GET" && !hasID:
		return "list"
	case method == "GET" && hasID:
		return "get"
	case method == "POST" && !hasID:
		return "create"
	case method == "POST" && hasID:
		if operationID != "" {
			return naming.ToSnakeCase(operationID)
		}
		return "create"
	case method == "PUT":
		return "update"
	case method == "DELETE":
		return "delete"
	case method == "PATCH":
		return "update"
	default:
		if operationID != "" {
			return naming.ToSnakeCase(operationID)
		}
		return strings.ToLower(method)
	}
}

func splitPath(path string) []string {
	var segments []string
	for _, s := range strings.Split(path, "/") {
		s = strings.TrimSpace(s)
		if s != "" {
			segments = append(segments, s)
		}
	}
	return segments
}

func isParam(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func stripBraces(segment string) string {
	return strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
}

func hasAction(actions []*Action, name string) bool {
	for _, a := range actions {
		if a.Name == name {
			return true
		}
	}
	return false
}

func lastNonEmpty(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return ss[len(ss)-1]
}

func sortedResources(resources map[string]*Resource) []*Resource {
	var result []*Resource
	for _, r := range resources {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
