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
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/relychan/openapitools/oaschema"
	"github.com/relychan/openapitools/openapiclient/handler"
	"github.com/relychan/openapitools/openapiclient/handler/proxyhandler"
)

type nodeType uint8

const (
	ntStatic   nodeType = iota // /home
	ntRegexp                   // /{id:[0-9]+}
	ntParam                    // /{user}
	ntCatchAll                 // /api/v1/*
)

// Route holds parameter values from the request path.
type Route struct {
	Pattern     string
	Method      *MethodHandler
	ParamValues map[string]string
}

// MethodHandler represents a handler data for a method.
type MethodHandler struct {
	Handler  proxyhandler.ProxyHandler
	Security []*base.SecurityRequirement
}

// Node presents the route tree to organize the recursive route structure.
type Node struct { //nolint:recvcheck
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
	pattern string,
	operations *highv3.PathItem,
	options *proxyhandler.InsertRouteOptions,
) (*Node, error) {
	node, err := n.insertChildNode(pattern, operations, nil, options)
	if err != nil {
		return nil, err
	}

	if node != nil && node.pattern == "" {
		node.pattern = pattern
	}

	return node, err
}

func (n *Node) FindRoute(path string, method string) *Route {
	route := &Route{
		ParamValues: map[string]string{},
	}

	// Find the routing handlers for the path
	m, pattern := route.findRouteRecursive(strings.TrimLeft(path, "/"), method, n)
	if m == nil {
		return nil
	}

	route.Method = m
	route.Pattern = pattern

	return route
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
		handlers, err := createMethods(pattern, operations, paramKeys, options)
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

		handlers, err := createMethods(pattern, operations, nil, options)
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
		return n.insertChildParamNode(search, operations, paramKeys, options)
	}

	// Static nodes fall below here.
	// Determine longest prefix of the search key on match.
	return n.insertChildStaticNode(search, operations, paramKeys, options)
}

func (n *Node) insertChildStaticNode(
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
		return child.insertChildNode(remain, operations, paramKeys, options)
	}

	// Insert or update the node's leaf handler
	handlers, err := createMethods(search, operations, paramKeys, options)
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

		return child.insertChildNode(remain, operations, paramKeys, options)
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

	return child.insertChildNode(remain, operations, paramKeys, options)
}

// Recursive edge traversal by checking all nodeTyp groups along the way.
// It's like searching through a multi-dimensional radix trie.
func (r *Route) findRouteRecursive( //nolint:gocognit
	search string,
	method string,
	node *Node,
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
					method, pattern := r.findRouteRecursive(remain, method, nd)
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
				)
				if method != nil {
					r.ParamValues[nd.key] = left

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

func createMethods( //nolint:cyclop,funlen
	pattern string,
	operations *highv3.PathItem,
	paramKeys []string,
	options *proxyhandler.InsertRouteOptions,
) (map[string]MethodHandler, error) {
	params := extractParametersFromOperationV3(operations, paramKeys)

	handlers := map[string]MethodHandler{}

	if operations.Get != nil {
		method := http.MethodGet

		h, err := handler.NewProxyHandler(operations.Get, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Get.Security,
		}
	}

	if operations.Post != nil {
		method := http.MethodPost

		h, err := handler.NewProxyHandler(operations.Post, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[http.MethodPost] = MethodHandler{
			Handler:  h,
			Security: operations.Post.Security,
		}
	}

	if operations.Put != nil {
		method := http.MethodPut

		h, err := handler.NewProxyHandler(operations.Put, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return nil, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Put.Security,
		}
	}

	if operations.Patch != nil {
		method := http.MethodPatch

		h, err := handler.NewProxyHandler(operations.Patch, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return handlers, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Patch.Security,
		}
	}

	if operations.Delete != nil {
		method := http.MethodDelete

		h, err := handler.NewProxyHandler(operations.Delete, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return handlers, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Delete.Security,
		}
	}

	if operations.Head != nil {
		method := http.MethodHead

		h, err := handler.NewProxyHandler(operations.Head, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return handlers, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Head.Security,
		}
	}

	if operations.Options != nil {
		method := http.MethodOptions

		h, err := handler.NewProxyHandler(
			operations.Options,
			&proxyhandler.NewProxyHandlerOptions{
				Method:     method,
				Parameters: params,
				GetEnv:     options.GetEnv,
			},
		)
		if err != nil {
			return handlers, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Options.Security,
		}
	}

	if operations.Query != nil {
		method := "QUERY"

		h, err := handler.NewProxyHandler(operations.Query, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return handlers, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Query.Security,
		}
	}

	if operations.Trace != nil {
		method := http.MethodTrace

		h, err := handler.NewProxyHandler(operations.Trace, &proxyhandler.NewProxyHandlerOptions{
			Method:     method,
			Parameters: params,
			GetEnv:     options.GetEnv,
		})
		if err != nil {
			return handlers, newInvalidOperationMetadataError(method, pattern, err)
		}

		handlers[method] = MethodHandler{
			Handler:  h,
			Security: operations.Trace.Security,
		}
	}

	if operations.AdditionalOperations != nil {
		for iter := operations.AdditionalOperations.Oldest(); iter != nil; iter = iter.Next() {
			method := iter.Key
			op := iter.Value

			if op == nil {
				continue
			}

			h, err := handler.NewProxyHandler(op, &proxyhandler.NewProxyHandlerOptions{
				Method:     method,
				Parameters: params,
				GetEnv:     options.GetEnv,
			})
			if err != nil {
				return handlers, newInvalidOperationMetadataError(method, pattern, err)
			}

			handlers[method] = MethodHandler{
				Handler:  h,
				Security: op.Security,
			}
		}
	}

	return handlers, nil
}

func extractParametersFromOperationV3(
	operations *highv3.PathItem,
	paramKeys []string,
) []*highv3.Parameter {
	params := operations.Parameters
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Get)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Post)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Put)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Patch)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Delete)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Head)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Options)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Query)
	params = oaschema.ExtractCommonParametersOfOperation(params, operations.Trace)

	if operations.AdditionalOperations != nil {
		for iter := operations.AdditionalOperations.Oldest(); iter != nil; iter = iter.Next() {
			if iter.Value == nil {
				continue
			}

			params = oaschema.ExtractCommonParametersOfOperation(params, iter.Value)
		}
	}

	// validates and add unknown parameters from the request pattern
	for _, key := range paramKeys {
		if slices.ContainsFunc(params, func(param *highv3.Parameter) bool {
			return param.In == oaschema.InPath.String() && param.Name == key
		}) {
			continue
		}

		params = append(params, &highv3.Parameter{
			Name:     key,
			In:       oaschema.InPath.String(),
			Required: new(true),
			Schema: base.CreateSchemaProxy(&base.Schema{
				Type: []string{"string"},
			}),
		})
	}

	return params
}

// cut the first path of the url and parse the query param if exists. Ignore fragments.
func cutURLPath(search string) (string, string, url.Values, error) { //nolint:revive
	if search == "" {
		return search, "", nil, nil
	}

	var endPathIndex int

	maxLength := len(search)

L:
	for ; endPathIndex < maxLength; endPathIndex++ {
		c := search[endPathIndex]

		switch c {
		case '/', '#':
			break L
		case '?':
			if endPathIndex == maxLength-1 {
				return search[:endPathIndex], "", nil, nil
			}

			queryParams, err := url.ParseQuery(search[endPathIndex+1:])
			if err != nil {
				return "", "", nil, err
			}

			return search[:endPathIndex], "", queryParams, nil
		default:
		}
	}

	if endPathIndex == maxLength {
		return search, "", nil, nil
	}

	return search[0:endPathIndex], search[endPathIndex+1:], nil, nil
}
