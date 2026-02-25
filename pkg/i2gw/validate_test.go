package i2gw

import (
	"testing"

	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestValidateMaxItems(t *testing.T) {

	gwResources := &GatewayResources{
		Gateways: map[types.NamespacedName]gatewayv1.Gateway{
			{Namespace: "default", Name: "gw1"}: {
				Spec: gatewayv1.GatewaySpec{
					//Allowed Listeners is 64
					Listeners: make([]gatewayv1.Listener, 65),
				},
			},
		},
	}

	errs := ValidateMaxItems(gwResources)

	if len(errs) < 1 {
		t.Errorf("expected error, got %d", len(errs))
	}
}
