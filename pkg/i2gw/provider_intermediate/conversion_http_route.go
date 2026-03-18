/*
Copyright 2026 The Kubernetes Authors.

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

package providerir

import (
	emitterir "github.com/kgateway-dev/ingress2gateway/pkg/i2gw/emitter_intermediate"
	"k8s.io/apimachinery/pkg/types"
)

func applyProviderSpecificHTTPRouteIR(out *emitterir.HTTPRouteContext, in HTTPRouteContext) {
	out.RuleBackendSources = convertBackendSources(in.RuleBackendSources)

	ingx := in.ProviderSpecificIR.IngressNginx
	if ingx == nil {
		return
	}

	out.RegexLocationForHost = ingx.RegexLocationForHost
	out.RegexForcedByUseRegex = ingx.RegexForcedByUseRegex
	out.RegexForcedByRewrite = ingx.RegexForcedByRewrite

	if ingx.Policies != nil {
		out.PoliciesBySourceIngressName = make(map[string]emitterir.Policy, len(ingx.Policies))
		for ingressName, policy := range ingx.Policies {
			out.PoliciesBySourceIngressName[ingressName] = convertIngressNginxPolicy(policy)
		}
	}
}

func convertBackendSources(in [][]BackendSource) [][]emitterir.BackendSource {
	if in == nil {
		return nil
	}
	out := make([][]emitterir.BackendSource, len(in))
	for i, ruleSources := range in {
		out[i] = make([]emitterir.BackendSource, len(ruleSources))
		for j, src := range ruleSources {
			out[i][j] = emitterir.BackendSource{
				Ingress:        src.Ingress,
				Path:           src.Path,
				DefaultBackend: src.DefaultBackend,
			}
		}
	}
	return out
}

func convertIngressNginxPolicy(in IngressNginxPolicy) emitterir.Policy {
	return emitterir.Policy{
		ClientBodyBufferSize: in.ClientBodyBufferSize,
		ProxyBodySize:        in.ProxyBodySize,
		Cors:                 convertIngressNginxCorsPolicy(in.Cors),
		RateLimit:            convertIngressNginxRateLimitPolicy(in.RateLimit),
		ProxySendTimeout:     in.ProxySendTimeout,
		ProxyReadTimeout:     in.ProxyReadTimeout,
		ProxyConnectTimeout:  in.ProxyConnectTimeout,
		EnableAccessLog:      in.EnableAccessLog,
		ExtAuth:              convertIngressNginxExtAuthPolicy(in.ExtAuth),
		BasicAuth:            convertIngressNginxBasicAuthPolicy(in.BasicAuth),
		SessionAffinity:      convertIngressNginxSessionAffinityPolicy(in.SessionAffinity),
		LoadBalancing:        convertIngressNginxBackendLoadBalancingPolicy(in.LoadBalancing),
		BackendTLS:           convertIngressNginxBackendTLSPolicy(in.BackendTLS),
		BackendProtocol:      convertIngressNginxBackendProtocol(in.BackendProtocol),
		SSLRedirect:          in.SSLRedirect,
		RewriteTarget:        in.RewriteTarget,
		UseRegexPaths:        in.UseRegexPaths,
		RuleBackendSources:   convertIngressNginxPolicyIndices(in.RuleBackendSources),
		Backends:             convertIngressNginxBackends(in.Backends),
	}
}

func convertIngressNginxPolicyIndices(in []IngressNginxPolicyIndex) []emitterir.PolicyIndex {
	if in == nil {
		return nil
	}
	out := make([]emitterir.PolicyIndex, len(in))
	for i := range in {
		out[i] = emitterir.PolicyIndex{
			Rule:    in[i].Rule,
			Backend: in[i].Backend,
		}
	}
	return out
}

func convertIngressNginxCorsPolicy(in *IngressNginxCorsPolicy) *emitterir.CorsPolicy {
	if in == nil {
		return nil
	}
	return &emitterir.CorsPolicy{
		Enable:           in.Enable,
		AllowOrigin:      cloneStringSlice(in.AllowOrigin),
		AllowCredentials: in.AllowCredentials,
		AllowHeaders:     cloneStringSlice(in.AllowHeaders),
		ExposeHeaders:    cloneStringSlice(in.ExposeHeaders),
		AllowMethods:     cloneStringSlice(in.AllowMethods),
		MaxAge:           in.MaxAge,
	}
}

func convertIngressNginxExtAuthPolicy(in *IngressNginxExtAuthPolicy) *emitterir.ExtAuthPolicy {
	if in == nil {
		return nil
	}
	return &emitterir.ExtAuthPolicy{
		AuthURL:         in.AuthURL,
		ResponseHeaders: cloneStringSlice(in.ResponseHeaders),
	}
}

func convertIngressNginxBasicAuthPolicy(in *IngressNginxBasicAuthPolicy) *emitterir.BasicAuthPolicy {
	if in == nil {
		return nil
	}
	return &emitterir.BasicAuthPolicy{
		SecretName: in.SecretName,
		AuthType:   in.AuthType,
	}
}

func convertIngressNginxSessionAffinityPolicy(in *IngressNginxSessionAffinityPolicy) *emitterir.SessionAffinityPolicy {
	if in == nil {
		return nil
	}
	return &emitterir.SessionAffinityPolicy{
		CookieName:     in.CookieName,
		CookiePath:     in.CookiePath,
		CookieDomain:   in.CookieDomain,
		CookieSameSite: in.CookieSameSite,
		CookieExpires:  in.CookieExpires,
		CookieSecure:   in.CookieSecure,
	}
}

func convertIngressNginxBackendTLSPolicy(in *IngressNginxBackendTLSPolicy) *emitterir.BackendTLSPolicy {
	if in == nil {
		return nil
	}
	return &emitterir.BackendTLSPolicy{
		SecretName: in.SecretName,
		Verify:     in.Verify,
		Hostname:   in.Hostname,
	}
}

func convertIngressNginxBackendLoadBalancingPolicy(in *IngressNginxBackendLoadBalancingPolicy) *emitterir.BackendLoadBalancingPolicy {
	if in == nil {
		return nil
	}
	return &emitterir.BackendLoadBalancingPolicy{
		Strategy: emitterir.LoadBalancingStrategy(in.Strategy),
	}
}

func convertIngressNginxRateLimitPolicy(in *IngressNginxRateLimitPolicy) *emitterir.RateLimitPolicy {
	if in == nil {
		return nil
	}
	return &emitterir.RateLimitPolicy{
		Limit:           in.Limit,
		Unit:            emitterir.RateLimitUnit(in.Unit),
		BurstMultiplier: in.BurstMultiplier,
	}
}

func convertIngressNginxBackendProtocol(in *IngressNginxBackendProtocol) *emitterir.BackendProtocol {
	if in == nil {
		return nil
	}
	value := emitterir.BackendProtocol(*in)
	return &value
}

func convertIngressNginxBackends(in map[types.NamespacedName]IngressNginxBackend) map[types.NamespacedName]emitterir.Backend {
	if in == nil {
		return nil
	}
	out := make(map[types.NamespacedName]emitterir.Backend, len(in))
	for key, backend := range in {
		out[key] = emitterir.Backend{
			Namespace: backend.Namespace,
			Name:      backend.Name,
			Port:      backend.Port,
			Host:      backend.Host,
			Protocol:  convertIngressNginxBackendProtocol(backend.Protocol),
		}
	}
	return out
}

func cloneStringSlice(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
