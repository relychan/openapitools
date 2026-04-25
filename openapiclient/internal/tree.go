// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package internal implements internal functionality for the proxy client.
package internal

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/goutils"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/oasvalidator"
	"github.com/relychan/openapitools/oasvalidator/parameter"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
)

type nodeType uint8

const (
	ntStatic   nodeType = iota // /home
	ntRegexp                   // /{id:[0-9]+}
	ntParam                    // /{user}
	ntCatchAll                 // /api/v1/*
)

// MethodHandler represents a handler data for a method.
type MethodHandler struct {
	Operation *highv3.Operation
	Handler   proxyhandler.ProxyHandler
}

// Node presents the route tree to organize the recursive route structure.
type Node struct {
	handlers map[string]MethodHandler

	// regexp matcher for regexp nodes
	rex *regexp.Regexp

	// key represents the static key
	key string

	// pattern is the full pattern of the leaf
	pattern string

	// child nodes should be stored in-order for iteration,
	// in groups of the node type.
	children [ntCatchAll + 1]nodes

	// node type: static, regexp, param, catchAll
	typ nodeType
}

// InsertRoute parses the route pattern into tree nodes and creates handlers.
func (n *Node) InsertRoute(
	document *highv3.Document,
	pattern string,
	operations *highv3.PathItem,
	options *proxyhandler.InsertRouteOptions,
) (*Node, error) {
	node, err := n.insertChildNode(document, pattern, operations, nil, options)
	if err != nil {
		return nil, err
	}

	if node != nil && node.pattern == "" {
		node.pattern = pattern
	}

	return node, err
}

func (n *Node) FindRoute(path string, method string) (*Route, *httperror.HTTPError) {
	route := &Route{}
	rawParams := make(map[string]string)

	// Find the routing handlers for the path
	m, pattern := route.findRouteRecursive(
		strings.TrimLeft(path, "/"),
		method,
		n,
		rawParams,
	)

	if m == nil {
		return nil, httperror.NewNotFoundError()
	}

	route.Method = m
	route.Pattern = pattern

	if m.Operation != nil {
		params, errs := validateURLParams(m.Operation, rawParams)
		if len(errs) > 0 {
			return nil, httperror.NewBadRequestError(errs...)
		}

		route.ParamValues = params
	}

	return route, nil
}

// String implements the fmt.Stringer interface to print debug data.
func (n Node) String() string {
	switch n.typ {
	case ntCatchAll:
		return "*"
	case ntStatic:
		return n.key
	case ntParam:
		return "{" + n.key + "}"
	case ntRegexp:
		return "{" + n.key + ":" + n.rex.String() + "}"
	default:
		return ""
	}
}

func (n *Node) insertChildNode(
	document *highv3.Document,
	pattern string,
	operations *highv3.PathItem,
	paramKeys []string,
	options *proxyhandler.InsertRouteOptions,
) (*Node, error) {
	search := strings.TrimPrefix(pattern, "/")

	// Handle key exhaustion
	if search == "" {
		childIndex := slices.IndexFunc(n.children[ntStatic], func(child *Node) bool {
			return child.key == ""
		})

		var child *Node

		if childIndex >= 0 {
			child = n.children[ntStatic][childIndex]
		} else {
			child = &Node{
				typ: ntStatic,
			}
		}

		// Insert or update the node's leaf handler
		handlers, err := createMethods(document, pattern, operations, paramKeys, options)
		if err != nil || len(handlers) == 0 {
			return nil, err
		}

		if len(handlers) > 0 {
			if len(child.handlers) > 0 {
				return nil, ErrDuplicatedRoutingPattern
			}

			child.handlers = handlers

			// insert new node
			if childIndex < 0 {
				n.children[ntStatic] = append(n.children[ntStatic], child)
				n.children[ntStatic].Sort()
			}

			return child, nil
		}

		return nil, nil
	}

	if search[0] == '*' {
		// wildcard must be placed at the edge.
		if len(search) > 1 {
			return nil, ErrWildcardMustBeLast
		}

		if len(n.children[ntCatchAll]) > 0 {
			return nil, ErrDuplicatedRoutingPattern
		}

		handlers, err := createMethods(document, pattern, operations, nil, options)
		if err != nil || len(handlers) == 0 {
			return nil, err
		}

		if len(handlers) == 0 {
			return nil, nil
		}

		child := &Node{
			typ:      ntCatchAll,
			handlers: handlers,
		}

		n.children[ntCatchAll] = append(n.children[ntCatchAll], child)

		return child, nil
	}

	// We're going to be searching for a wild node next,
	// in this case, we need to get the tail
	if search[0] == '{' {
		return n.insertChildParamNode(document, search, operations, paramKeys, options)
	}

	// Static nodes fall below here.
	// Determine longest prefix of the search key on match.
	return n.insertChildStaticNode(document, search, operations, paramKeys, options)
}

func (n *Node) insertChildStaticNode(
	document *highv3.Document,
	search string,
	operations *highv3.PathItem,
	paramKeys []string,
	options *proxyhandler.InsertRouteOptions,
) (*Node, error) {
	rawSegment, remain, _, err := cutURLPath(search)
	if err != nil {
		return nil, err
	}

	childIndex := slices.IndexFunc(n.children[ntStatic], func(child *Node) bool {
		return child.key == rawSegment
	})

	var child *Node

	if childIndex >= 0 {
		child = n.children[ntStatic][childIndex]
	} else {
		child = &Node{
			typ: ntStatic,
			key: rawSegment,
		}

		n.children[ntStatic] = append(n.children[ntStatic], child)
		n.children[ntStatic].Sort()
	}

	if remain != "" {
		return child.insertChildNode(document, remain, operations, paramKeys, options)
	}

	// Insert or update the node's leaf handler
	handlers, err := createMethods(document, search, operations, paramKeys, options)
	if err != nil || len(handlers) == 0 {
		return nil, err
	}

	if len(handlers) > 0 {
		child.handlers = handlers

		return child, nil
	}

	return nil, nil
}

func (n *Node) insertChildParamNode(
	document *highv3.Document,
	search string,
	operations *highv3.PathItem,
	paramKeys []string,
	options *proxyhandler.InsertRouteOptions,
) (*Node, error) {
	rawSegment, remain, _ := strings.Cut(search, "/")

	segment, err := patNextSegment(rawSegment)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rawSegment, err)
	}

	paramKeys = append(paramKeys, segment.ParamName)

	if segment.NodeType == ntParam {
		childIndex := slices.IndexFunc(n.children[ntParam], func(child *Node) bool {
			return child.key == segment.ParamName
		})

		var child *Node

		if childIndex >= 0 {
			child = n.children[ntParam][childIndex]
		} else {
			child = &Node{
				typ: ntParam,
				key: segment.ParamName,
			}

			n.children[ntParam] = append(n.children[ntParam], child)
			n.children[ntParam].Sort()
		}

		return child.insertChildNode(document, remain, operations, paramKeys, options)
	}

	childIndex := slices.IndexFunc(n.children[ntRegexp], func(child *Node) bool {
		return child.key == segment.ParamName && child.rex.String() == segment.Regexp
	})

	var child *Node

	if childIndex >= 0 {
		child = n.children[ntRegexp][childIndex]
	} else {
		rex, err := regexp.Compile(segment.Regexp)
		if err != nil {
			return nil, err
		}

		child = &Node{
			typ: ntRegexp,
			key: segment.ParamName,
			rex: rex,
		}

		n.children[ntRegexp] = append(n.children[ntRegexp], child)
		n.children[ntRegexp].Sort()
	}

	return child.insertChildNode(document, remain, operations, paramKeys, options)
}

// Recursive edge traversal by checking all nodeTyp groups along the way.
// It's like searching through a multi-dimensional radix trie.
func (r *Route) findRouteRecursive( //nolint:gocognit
	search string,
	method string,
	node *Node,
	params map[string]string,
) (*MethodHandler, string) {
	left, remain, _ := strings.Cut(search, "/")

	for t, nds := range node.children {
		ntyp := nodeType(t)

		if len(nds) == 0 {
			continue
		}

		switch ntyp {
		case ntStatic:
			for _, nd := range nds {
				if nd.key != left {
					continue
				}

				if remain == "" {
					method := nd.findMethod(method)
					if method != nil {
						return method, nd.pattern
					}
				} else {
					method, pattern := r.findRouteRecursive(remain, method, nd, params)
					if method != nil {
						return method, pattern
					}
				}
			}
		case ntParam, ntRegexp:
			// short-circuit and return no matching route for empty param values
			if search == "" {
				continue
			}

			// serially loop through each node grouped by the tail delimiter
			for _, nd := range nds {
				if nd.rex != nil && !nd.rex.MatchString(left) {
					continue
				}

				method, pattern := r.findRouteRecursive(
					remain,
					method,
					nd,
					params,
				)
				if method != nil {
					params[nd.key] = left

					return method, pattern
				}
			}
		default:
			// catch-all nodes
			return nds[0].findMethod(method), nds[0].pattern
		}
	}

	return nil, ""
}

func (n *Node) findMethod(name string) *MethodHandler {
	if len(n.handlers) == 0 {
		return nil
	}

	h, ok := n.handlers[name]
	if !ok {
		return nil
	}

	return &h
}

type nodes []*Node

// Sort the list of nodes by label.
func (ns nodes) Sort() {
	slices.SortFunc(ns, func(a, b *Node) int {
		if a.typ == b.typ {
			return strings.Compare(a.key, b.key)
		}

		return int(a.typ) - int(b.typ)
	})
}

type patNextSegmentResult struct {
	ParamName string
	Regexp    string
	NodeType  nodeType
}

// patNextSegment returns the next segment details from a pattern.
func patNextSegment(pattern string) (*patNextSegmentResult, error) {
	var endIndex, regexIndex int

	for i := 1; i < len(pattern); i++ {
		c := pattern[i]

		switch c {
		case ':':
			regexIndex = i
		case '}':
			endIndex = i
		default:
		}
	}

	if endIndex == 0 {
		return nil, ErrMissingClosingBracket
	}

	// Param/Regexp pattern is next
	nt := ntParam

	var rePattern string

	paramName := pattern[1:endIndex]

	if regexIndex > 0 {
		if regexIndex >= endIndex {
			return nil, ErrInvalidRegexpPatternParamInRoute
		}

		nt = ntRegexp

		paramName = pattern[1:regexIndex]
		rePattern = pattern[regexIndex+1 : endIndex]

		// make sure that the regular expression evaluates the exact match.
		if rePattern[0] != '^' {
			rePattern = "^" + rePattern
		}

		if rePattern[len(rePattern)-1] != '$' {
			rePattern += "$"
		}
	}

	if paramName == "" {
		return nil, ErrParamKeyRequired
	}

	result := &patNextSegmentResult{
		NodeType:  nt,
		ParamName: paramName,
		Regexp:    rePattern,
	}

	if endIndex == len(pattern)-1 {
		return result, nil
	}

	switch pattern[endIndex+1] {
	case '?', '#':
		// Leaf node with query or fragment params are valid.
		return result, nil
	default:
		return nil, ErrInvalidParamPattern
	}
}

func validateURLParams(
	operation *highv3.Operation,
	values map[string]string,
) (map[string]any, []httperror.ValidationError) {
	result := goutils.ToAnyMap(values)

	if operation == nil || len(operation.Parameters) == 0 {
		return result, nil
	}

	var errs []httperror.ValidationError

	for _, param := range operation.Parameters {
		if param.In != oaschema.InPath.String() {
			continue
		}

		rawValue, ok := values[param.Name]
		if !ok {
			if param.Required != nil && *param.Required {
				errs = append(errs, httperror.ValidationError{
					Code:      oasvalidator.ErrCodeInvalidURLParam,
					Detail:    "Parameter is required",
					Parameter: param.Name,
				})
			}

			continue
		}

		value, decodeErrors := parameter.DecodePathValue(param, rawValue)
		if len(decodeErrors) > 0 {
			errs = append(errs, decodeErrors...)
		} else {
			result[param.Name] = value
		}
	}

	return result, errs
}
