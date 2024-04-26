/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openapi

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
)

const (
	HostSeparator  = ","
	ParamSeparator = ","

	HTTPRouteRulesMax      = 16
	HTTPRouteMatchesMax    = 8
	HTTPRouteMatchesMaxMax = HTTPRouteRulesMax * HTTPRouteMatchesMax
)

// converter implements the ToGatewayAPI function of i2gw.ResourceConverter interface.
type converter struct {
	conf *i2gw.ProviderConf

	featureParsers                []i2gw.FeatureParser
	implementationSpecificOptions i2gw.ProviderImplementationSpecificOptions
}

// newConverter returns an ingress-nginx converter instance.
func newConverter(conf *i2gw.ProviderConf) *converter {
	return &converter{
		conf:           conf,
		featureParsers: []i2gw.FeatureParser{
			// The list of feature parsers comes here.
		},
		implementationSpecificOptions: i2gw.ProviderImplementationSpecificOptions{
			// The list of the implementationSpecific ingress fields options comes here.
		},
	}
}

func (c *converter) convert(storage *storage) (i2gw.GatewayResources, field.ErrorList) {
	gatewayResources := i2gw.GatewayResources{
		Gateways:        make(map[types.NamespacedName]gatewayv1.Gateway),
		HTTPRoutes:      make(map[types.NamespacedName]gatewayv1.HTTPRoute),
		TLSRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TLSRoute),
		TCPRoutes:       make(map[types.NamespacedName]gatewayv1alpha2.TCPRoute),
		ReferenceGrants: make(map[types.NamespacedName]gatewayv1beta1.ReferenceGrant),
	}

	var errors field.ErrorList

	for _, spec := range storage.getResources() {
		httpRoutes := toHTTPRoutes(spec, errors)
		for _, httpRoute := range httpRoutes {
			gatewayResources.HTTPRoutes[types.NamespacedName{Name: httpRoute.GetName(), Namespace: httpRoute.GetNamespace()}] = httpRoute
		}
		// TODO: build Gateways
		// TODO: add parentRefs to HTTPRoutes
	}

	return gatewayResources, errors
}

type httpRouteRuleMatcher struct {
	path    string
	method  string
	headers string
	params  string
}

type httpRouteRuleMatchers []httpRouteRuleMatcher

func (m httpRouteRuleMatchers) Len() int      { return len(m) }
func (m httpRouteRuleMatchers) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m httpRouteRuleMatchers) Less(i, j int) bool {
	if m[i].path != m[j].path {
		return m[i].path < m[j].path
	}
	return m[i].method < m[j].method
}

type httpRouteMatcher struct {
	host string
	httpRouteRuleMatcher
}

func toHTTPRoutes(spec *openapi3.T, errors field.ErrorList) []gatewayv1.HTTPRoute {
	var matchers []httpRouteMatcher

	servers := spec.Servers
	if len(servers) == 0 {
		servers = openapi3.Servers{{URL: "/"}}
	}

	paths := spec.Paths.Map()
	for _, relativePath := range spec.Paths.InMatchingOrder() {
		pathItem := paths[relativePath]
		matchers = append(matchers, pathItemToHTTPMatchers(pathItem, relativePath, servers, errors)...)
	}

	hostsByHTTPRouteRuleMatcher := make(map[httpRouteRuleMatcher][]string)
	for _, matcher := range matchers {
		hostsByHTTPRouteRuleMatcher[matcher.httpRouteRuleMatcher] = append(hostsByHTTPRouteRuleMatcher[matcher.httpRouteRuleMatcher], matcher.host)
	}

	var hostGroups []string
	httpRouteRuleMatchersByHosts := make(map[string]httpRouteRuleMatchers)
	for matcher, hosts := range hostsByHTTPRouteRuleMatcher {
		group := strings.Join(hosts, HostSeparator)
		if _, exists := httpRouteRuleMatchersByHosts[group]; !exists {
			hostGroups = append(hostGroups, group)
		}
		httpRouteRuleMatchersByHosts[group] = append(httpRouteRuleMatchersByHosts[group], matcher)
	}

	var routes []gatewayv1.HTTPRoute

	// sort host groups for deterministic output
	sort.Strings(hostGroups)

	i := 0
	for _, group := range hostGroups {
		hosts := strings.Split(group, HostSeparator)
		matchers := httpRouteRuleMatchersByHosts[group]

		// sort hostnames and matchers for deterministic output inside each route object
		sort.Strings(hosts)
		sort.Sort(matchers)

		nMatchers := len(matchers)
		nRoutes := nMatchers / HTTPRouteMatchesMaxMax
		if nMatchers%HTTPRouteMatchesMaxMax != 0 {
			nRoutes++
		}
		for j := 0; j < nRoutes; j++ {
			routeName := fmt.Sprintf("route-%d-%d", i+1, j+1)
			// TODO: name the route after the spec
			last := (j + 1) * HTTPRouteMatchesMaxMax
			if last > nMatchers {
				last = nMatchers
			}
			routes = append(routes, toHTTPRoute(routeName, hosts, matchers[j*HTTPRouteMatchesMaxMax:last]))
		}
		i++
	}

	return routes
}

func toHTTPRoute(name string, hostnames []string, matchers httpRouteRuleMatchers) gatewayv1.HTTPRoute {
	route := gatewayv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1.GroupVersion.String(),
			Kind:       "HTTPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: toHTTPRouteRules(matchers),
		},
	}
	if len(hostnames) > 1 || !slices.Contains(hostnames, "") {
		route.Spec.Hostnames = Map(hostnames, toGatewayAPIHostname)
	}
	return route
}

func toHTTPRouteRules(matchers httpRouteRuleMatchers) []gatewayv1.HTTPRouteRule {
	var rules []gatewayv1.HTTPRouteRule
	nMatches := len(matchers)
	nRules := nMatches / HTTPRouteMatchesMax
	if len(matchers)%HTTPRouteMatchesMax != 0 {
		nRules++
	}
	for i := 0; i < nRules; i++ {
		rule := gatewayv1.HTTPRouteRule{}
		offfset := i * HTTPRouteMatchesMax
		for j := 0; j < HTTPRouteMatchesMax && offfset+j < nMatches; j++ {
			matcher := matchers[offfset+j]
			ruleMatch := gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  common.PtrTo(gatewayv1.PathMatchExact),
					Value: &matcher.path,
				},
				Method: common.PtrTo(gatewayv1.HTTPMethod(matcher.method)),
			}
			if matcher.headers != "" {
				ruleMatch.Headers = Map(strings.Split(matcher.headers, ParamSeparator), func(header string) gatewayv1.HTTPHeaderMatch {
					return gatewayv1.HTTPHeaderMatch{
						Name:  gatewayv1.HTTPHeaderName(header),
						Type:  common.PtrTo(gatewayv1.HeaderMatchExact),
					}
				})
			}
			if matcher.params != "" {
				ruleMatch.QueryParams = Map(strings.Split(matcher.params, ParamSeparator), func(param string) gatewayv1.HTTPQueryParamMatch {
					return gatewayv1.HTTPQueryParamMatch{
						Name: gatewayv1.HTTPHeaderName(param),
						Type: common.PtrTo(gatewayv1.QueryParamMatchExact),
					}
				})
			}
			rule.Matches = append(rule.Matches, ruleMatch)
		}
		// TODO: backendRefs
		rules = append(rules, rule)
	}
	return rules
}

func pathItemToHTTPMatchers(pathItem *openapi3.PathItem, relativePath string, servers openapi3.Servers, errors field.ErrorList) []httpRouteMatcher {
	var matchers []httpRouteMatcher

	if len(pathItem.Servers) > 0 {
		servers = pathItem.Servers
	}

	operations := map[string]*openapi3.Operation{
		"CONNECT": pathItem.Connect,
		"DELETE":  pathItem.Delete,
		"GET":     pathItem.Get,
		"HEAD":    pathItem.Head,
		"OPTIONS": pathItem.Options,
		"PATCH":   pathItem.Patch,
		"POST":    pathItem.Post,
		"PUT":     pathItem.Put,
		"TRACE":   pathItem.Trace,
	}

	for method, operation := range operations {
		if operation == nil {
			continue
		}
		matchers = append(matchers, operationToHTTPMatchers(operation, relativePath, method, pathItem.Parameters, servers, errors)...)
	}

	return matchers
}

func operationToHTTPMatchers(operation *openapi3.Operation, relativePath string, method string, parameters openapi3.Parameters, servers openapi3.Servers, errors field.ErrorList) []httpRouteMatcher {
	if operation.Servers != nil && len(*operation.Servers) > 0 {
		servers = *operation.Servers
	}

	if operation.Parameters != nil {
		parameters = operation.Parameters
	}

  var expandedServers []openapi3.Server
	expandedHosts := make(map[string]struct{})
	for _, server := range servers {
		for _, expandedServer := range expandServerVariables(*server) {
			basePath, err := expandedServer.BasePath()
			if err != nil {
				errors = append(errors, field.Invalid(field.NewPath("servers"), expandedServer, err.Error()))
			}
			host := uriToHostname(expandedServer.URL) + basePath
			if _, exists := expandedHosts[host]; !exists {
				expandedServers = append(expandedServers, expandedServer)
				expandedHosts[host] = struct{}{}
			}
		}
	}

	return Map(expandedServers, toHTTPMatcher(relativePath, method, parameters, errors))
}

func toHTTPMatcher(relativePath string, method string, parameters openapi3.Parameters, errors field.ErrorList) func(server openapi3.Server) httpRouteMatcher {
	paramInFunc := func(in string) func(p *openapi3.ParameterRef) bool {
		return func(p *openapi3.ParameterRef) bool {
			return p.Value != nil && p.Value.Required && p.Value.In == in
		}
	}
	paramNameFunc := func(p *openapi3.ParameterRef) string { return p.Value.Name }

	headers := strings.Join(Map(Filter(parameters, paramInFunc("header")), paramNameFunc), ParamSeparator)
	params := strings.Join(Map(Filter(parameters, paramInFunc("query")), paramNameFunc), ParamSeparator)

	return func(server openapi3.Server) httpRouteMatcher {
		basePath, err := server.BasePath()
		if err != nil {
			errors = append(errors, field.Invalid(field.NewPath("servers"), server, err.Error()))
		}
		if basePath == "/" {
			basePath = ""
		}
		return httpRouteMatcher{
			host: uriToHostname(server.URL),
			httpRouteRuleMatcher: httpRouteRuleMatcher{
				path:    basePath + relativePath,
				method:  method,
				headers: headers,
				params:  params,
			},
		}
	}
}

func expandNonEnumServerVariables(server openapi3.Server) openapi3.Server {
	if len(server.Variables) == 0 {
		return server
	}
	// non-enum variables
	uri := server.URL
	variables := make(map[string]*openapi3.ServerVariable)
	for name, svar := range server.Variables {
		if len(svar.Enum) > 0 {
			variables[name] = svar
			continue
		}
		uri = strings.ReplaceAll(uri, "{"+name+"}", svar.Default)
	}
	return openapi3.Server{
		URL:       uri,
		Variables: variables,
	}
}

func expandServerVariables(server openapi3.Server) []openapi3.Server {
	servers := []openapi3.Server{expandNonEnumServerVariables(server)}
	for {
		var newServers []openapi3.Server
		for _, server := range servers {
			if len(server.Variables) == 0 {
				newServers = append(newServers, server)
				continue
			}
			var name string
			var svar *openapi3.ServerVariable
			for name, svar = range server.Variables { break }
			var uris []string
			for _, enum := range svar.Enum {
				uri := strings.ReplaceAll(server.URL, "{"+name+"}", enum)
				uris = append(uris, uri)
			}
			variables := make(map[string]*openapi3.ServerVariable, len(server.Variables)-1)
			for n, v := range server.Variables {
				if n != name {
					variables[n] = v
				}
			}
			for _, uri := range uris {
				newServers = append(newServers, openapi3.Server{
					URL:       uri,
					Variables: variables,
				})
			}
		}
		servers = newServers
		if slices.IndexFunc(servers, func(server openapi3.Server) bool { return len(server.Variables) > 0 }) == -1 {
			break
		}
	}
	return servers
}

func uriToHostname(uri string) string {
	host := uri
	if strings.Contains(host, "://") {
		host = strings.SplitN(host, "://", 2)[1]
	}
	return strings.SplitN(host, "/", 2)[0]
}

func toGatewayAPIHostname(hostname string) gatewayv1.Hostname {
	return gatewayv1.Hostname(hostname)
}
