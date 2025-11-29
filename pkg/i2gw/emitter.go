package i2gw

import (
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/intermediate"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// EmitterByName is a map of Emitter by a emitter name.
// Different Emitter should add their implement at startup.
var EmitterByName = map[EmitterName]Emitter{}

// EmitterName is a string alias that stores the concrete Emitter name.
type EmitterName string

// The Emitter interface specifies conversion functions from IR
// into Gateway and Gateway extensions.
type Emitter interface {

	// ToGatewayResources converts stored IR with the Provider into
	// Gateway API resources and extensions
	ToGatewayResources(ir intermediate.IR) (GatewayResources, field.ErrorList)
}
