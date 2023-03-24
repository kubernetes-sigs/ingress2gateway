package translator

import (
	"testing"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCanaryRetrieveAnnotations(t *testing.T) {
	testcases := []struct {
		name         string
		provider     IngressProvider
		ingress      networkingv1.Ingress
		expectCanary *canaryGroup
		expectError  bool
	}{
		{
			name:     "unsupported provider",
			provider: IngressProvider("unknown"),
			ingress: networkingv1.Ingress{
				ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{}},
			},
			expectError: true,
		}, {
			name:     "retrieve canary from annotations when canary is false",
			provider: IngressNginxIngressProvider,
			ingress: networkingv1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":                 "false",
						"nginx.ingress.kubernetes.io/canary-by-header":       "traffic-split",
						"nginx.ingress.kubernetes.io/canary-by-header-value": "v1",
					},
				},
			},
			expectError:  false,
			expectCanary: &canaryGroup{},
		}, {
			name:     "retrieve canary header/value from annotations when canary is true",
			provider: IngressNginxIngressProvider,
			ingress: networkingv1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":                 "true",
						"nginx.ingress.kubernetes.io/canary-by-header":       "traffic-split",
						"nginx.ingress.kubernetes.io/canary-by-header-value": "v1",
					},
				},
			},
			expectError: false,
			expectCanary: &canaryGroup{
				enable:      true,
				headerKey:   "traffic-split",
				headerValue: "v1",
			},
		}, {
			name:     "retrieve canary header/pattern from annotations when canary is true",
			provider: IngressNginxIngressProvider,
			ingress: networkingv1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":                   "true",
						"nginx.ingress.kubernetes.io/canary-by-header":         "traffic-split",
						"nginx.ingress.kubernetes.io/canary-by-header-value":   "v1",
						"nginx.ingress.kubernetes.io/canary-by-header-pattern": "*",
					},
				},
			},
			expectError: false,
			expectCanary: &canaryGroup{
				enable:           true,
				headerKey:        "traffic-split",
				headerRegexMatch: true,
				headerValue:      "*",
			},
		}, {
			name:     "retrieve canary weight from annotations when canary is true",
			provider: IngressNginxIngressProvider,
			ingress: networkingv1.Ingress{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/canary":                   "true",
						"nginx.ingress.kubernetes.io/canary-by-header":         "traffic-split",
						"nginx.ingress.kubernetes.io/canary-by-header-value":   "v1",
						"nginx.ingress.kubernetes.io/canary-by-header-pattern": "*",
						"nginx.ingress.kubernetes.io/canary-weight":            "20",
						"nginx.ingress.kubernetes.io/canary-weight-total":      "80",
					},
				},
			},
			expectError: false,
			expectCanary: &canaryGroup{
				enable:           true,
				headerKey:        "traffic-split",
				headerRegexMatch: true,
				headerValue:      "*",
				weight:           20,
				weightTotal:      80,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cg := &canaryGroup{}
			actual, err := cg.retrieveAnnotations(tc.provider, tc.ingress)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectCanary, actual)
		})
	}
}
