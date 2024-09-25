package cilium

import (
	"context"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
)

// converter implements the i2gw.CustomResourceReader interface.
type resourceReader struct {
	conf *i2gw.ProviderConf
}

// newResourceReader returns a resourceReader instance.
func newResourceReader(conf *i2gw.ProviderConf) *resourceReader {
	return &resourceReader{
		conf: conf,
	}
}

func (r *resourceReader) ReadResourcesFromCluster(ctx context.Context) error {
	// read example-gateway related resources from the cluster.
	return nil
}

func (r *resourceReader) ReadResourcesFromFiles(ctx context.Context, filename string) error {
	// read example-gateway related resources from the file.
	return nil
}
