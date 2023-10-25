package crds

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"k8s.io/apimachinery/pkg/util/validation/field"

	configurationv1beta1 "github.com/kong/kubernetes-ingress-controller/v2/pkg/apis/configuration/v1beta1"
)

// TODO: comment
func TcpIngressToGatewayAPI(tcpIngresses []configurationv1beta1.TCPIngress) (i2gw.GatewayResources, field.ErrorList) {
	return i2gw.GatewayResources{}, nil
}
