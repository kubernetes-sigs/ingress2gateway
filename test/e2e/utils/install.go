/*
Copyright 2023 The Kubernetes Authors.

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

package utils

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"
)

func KindClusterExists(ctx context.Context, name string) bool {
	out, err := exec.CommandContext(ctx, "kind", "get", "clusters").CombinedOutput()
	if err != nil {
		log.Printf("WARN: kind get clusters failed (%v): %s", err, strings.TrimSpace(string(out)))
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

func pickLBRange(cidr string, count uint32) (net.IP, net.IP, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, err
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, nil, fmt.Errorf("only IPv4 supported, got %s", cidr)
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return nil, nil, fmt.Errorf("unexpected mask bits: %d", bits)
	}
	total := uint32(1) << uint32(32-ones)
	if total < count+10 {
		return nil, nil, fmt.Errorf("subnet too small for lb range: %s", cidr)
	}

	base := binary.BigEndian.Uint32(ipnet.IP.To4())
	last := base + total - 2
	first := last - count

	start := make(net.IP, 4)
	end := make(net.IP, 4)
	binary.BigEndian.PutUint32(start, first)
	binary.BigEndian.PutUint32(end, last)

	if !ipnet.Contains(start) || !ipnet.Contains(end) {
		return nil, nil, fmt.Errorf("computed range not within subnet: %s-%s not in %s", start, end, cidr)
	}
	return start, end, nil
}

func MustDockerKindSubnet(ctx context.Context) string {
	// The "kind" docker network may be dual-stack, so IPAM.Config can contain
	// both IPv6 and IPv4 subnets. We only support IPv4 for MetalLB ranges, so pick
	// the first IPv4 subnet we find.
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", "kind", "-f", "{{json .IPAM.Config}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("docker network inspect kind: %w: %s", err, string(out)))
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		panic("empty IPAM config from docker network inspect kind")
	}

	type ipamCfg struct {
		Subnet string `json:"Subnet"`
	}

	var cfgs []ipamCfg
	if err := json.Unmarshal([]byte(raw), &cfgs); err != nil {
		panic(fmt.Errorf("parse docker network IPAM config: %w: %s", err, raw))
	}

	for _, c := range cfgs {
		cidr := strings.TrimSpace(c.Subnet)
		if cidr == "" {
			continue
		}
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			return cidr
		}
	}

	panic(fmt.Errorf("no IPv4 subnet found in docker network %q IPAM config: %s", "kind", raw))
}

func InstallMetalLB(ctx context.Context, kubeContext, version string) {
	ver := EnvOrDefault("METALLB_VERSION", version)
	manifestURL := fmt.Sprintf("https://raw.githubusercontent.com/metallb/metallb/%s/config/manifests/metallb-native.yaml", ver)

	log.Printf("Installing MetalLB %s", ver)
	MustKubectl(ctx, kubeContext, "apply", "-f", manifestURL)

	MustKubectl(ctx, kubeContext, "-n", "metallb-system", "rollout", "status", "deploy/controller", "--timeout=3m")
	MustKubectl(ctx, kubeContext, "-n", "metallb-system", "rollout", "status", "ds/speaker", "--timeout=3m")

	cidr := MustDockerKindSubnet(ctx)
	start, end, err := pickLBRange(cidr, 50)
	if err != nil {
		panic(fmt.Errorf("pick LB range from %s: %w", cidr, err))
	}
	poolYAML := fmt.Sprintf(`
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kind-pool
  namespace: metallb-system
spec:
  addresses:
  - %s-%s
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kind-adv
  namespace: metallb-system
spec:
  ipAddressPools:
  - kind-pool
`, start, end)

	log.Printf("Configuring MetalLB IPAddressPool %s-%s (from kind subnet %s)", start, end, cidr)
	MustKubectlApplyStdin(ctx, kubeContext, poolYAML)
}

func InstallGatewayAPICRDs(ctx context.Context, kubeContext, version string) {
	ver := EnvOrDefault("GATEWAY_API_VERSION", version)
	url := fmt.Sprintf("https://github.com/kubernetes-sigs/gateway-api/releases/download/%s/experimental-install.yaml", ver)

	log.Printf("Installing Gateway API CRDs %s (experimental)", ver)
	MustKubectl(ctx, kubeContext, "apply", "--server-side=true", "-f", url)

	MustKubectl(ctx, kubeContext, "get", "crd", "gateways.gateway.networking.k8s.io")
	MustKubectl(ctx, kubeContext, "get", "crd", "httproutes.gateway.networking.k8s.io")
}

func InstallIngressNginx(ctx context.Context, kubeContext, version string) {
	ver := EnvOrDefault("INGRESS_NGINX_VERSION", version)
	url := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-%s/deploy/static/provider/cloud/deploy.yaml", ver)

	log.Printf("Installing ingress-nginx %s from %s", ver, url)
	MustKubectl(ctx, kubeContext, "apply", "-f", url)

	// Enable SSL passthrough for TLS passthrough tests
	log.Printf("Enabling SSL passthrough in ingress-nginx controller")
	// Patch the deployment to add the --enable-ssl-passthrough flag
	// If the flag already exists, the patch will fail, but we'll verify it's present
	if _, err := Kubectl(ctx, kubeContext, "-n", "ingress-nginx", "patch", "deployment", "ingress-nginx-controller",
		"--type=json",
		"-p", `[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--enable-ssl-passthrough"}]`); err != nil {
		// Check if the flag is already present (patch might fail if flag exists)
		argsOut, getErr := Kubectl(ctx, kubeContext, "-n", "ingress-nginx", "get", "deployment", "ingress-nginx-controller", "-o", "jsonpath={.spec.template.spec.containers[0].args}")
		if getErr == nil && strings.Contains(argsOut, "--enable-ssl-passthrough") {
			log.Printf("SSL passthrough flag already present in ingress-nginx controller")
		} else {
			log.Printf("Warning: Failed to enable SSL passthrough (patch error: %v)", err)
		}
	}

	MustKubectl(ctx, kubeContext, "-n", "ingress-nginx", "rollout", "status", "deploy/ingress-nginx-controller", "--timeout=1m")

	if _, err := WaitForServiceAddress(ctx, kubeContext, "ingress-nginx", "ingress-nginx-controller", 1*time.Minute); err != nil {
		panic(fmt.Errorf("ingress-nginx-controller external IP: %w", err))
	}
}

func ApplyEchoBackend(ctx context.Context, kubeContext, image string) {
	img := EnvOrDefault("ECHO_IMAGE", image)
	y := fmt.Sprintf(`
apiVersion: v1
kind: Service
metadata:
  name: echo-backend
  namespace: default
spec:
  selector:
    app: echo-backend
  ports:
  - name: http
    port: 8080
    targetPort: 3000
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-backend
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: echo-backend
  template:
    metadata:
      labels:
        app: echo-backend
    spec:
      containers:
      - name: echo-backend
        image: %s
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: GRPC_ECHO_SERVER
          value: "true"
        ports:
        - containerPort: 3000
        readinessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 2
          periodSeconds: 2
`, img)

	log.Printf("Deploying echo backend (%s)", img)
	MustKubectlApplyStdin(ctx, kubeContext, y)
	MustKubectl(ctx, kubeContext, "-n", "default", "rollout", "status", "deploy/echo-backend", "--timeout=5m")
}

func ApplyTLSBackend(ctx context.Context, kubeContext, image string) {
	img := EnvOrDefault("ECHO_IMAGE", image)

	// Generate TLS certificates for the backend
	CreateTLSSecret(ctx, kubeContext, "tls-secret", "ssl-passthrough.localdev.me")

	y := fmt.Sprintf(`
apiVersion: v1
kind: Service
metadata:
  name: tls-svc
  namespace: default
  labels:
    app: tls-svc
spec:
  ports:
  - port: 443
    targetPort: 8443
    protocol: TCP
  selector:
    app: backend-tls
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend-tls
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: backend-tls
      version: v1
  template:
    metadata:
      labels:
        app: backend-tls
        version: v1
    spec:
      containers:
      - image: %s
        imagePullPolicy: IfNotPresent
        name: backend-tls
        ports:
        - containerPort: 8443
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: SERVICE_NAME
          value: tls-svc
        - name: HTTPS_PORT
          value: "8443"
        - name: TLS_SERVER_CERT
          value: /etc/server-certs/tls.crt
        - name: TLS_SERVER_PRIVKEY
          value: /etc/server-certs/tls.key
        volumeMounts:
        - name: server-certs
          mountPath: /etc/server-certs
          readOnly: true
      volumes:
      - name: server-certs
        secret:
          secretName: tls-secret
`, img)

	log.Printf("Deploying TLS backend (%s)", img)
	MustKubectlApplyStdin(ctx, kubeContext, y)
	MustKubectl(ctx, kubeContext, "-n", "default", "rollout", "status", "deploy/backend-tls", "--timeout=5m")
}

func CreateTLSSecret(ctx context.Context, kubeContext, secretName, hostname string) {
	// Check if secret already exists
	if _, err := Kubectl(ctx, kubeContext, "get", "secret", secretName, "-n", "default"); err == nil {
		log.Printf("TLS secret %s already exists, skipping creation", secretName)
		return
	}

	// Generate self-signed certificate using openssl
	// Create a temporary directory for cert files
	tmpDir := "/tmp/i2g-tls-certs"
	MustRun(ctx, "mkdir", "-p", tmpDir)

	certFile := fmt.Sprintf("%s/tls.crt", tmpDir)
	keyFile := fmt.Sprintf("%s/tls.key", tmpDir)

	// Generate certificate valid for 365 days
	subject := fmt.Sprintf("/CN=%s/O=ingress2gateway", hostname)
	subjectAltName := fmt.Sprintf("subjectAltName=DNS:%s", hostname)
	MustRun(
		ctx,
		"openssl", "req",
		"-x509",
		"-nodes",
		"-days", "365",
		"-newkey", "rsa:2048",
		"-keyout", keyFile,
		"-out", certFile,
		"-subj", subject,
		"-addext", subjectAltName,
	)

	// Create Kubernetes secret from the certificate files
	cmd := exec.CommandContext(ctx, "kubectl",
		"--context", kubeContext,
		"create", "secret", "tls", secretName,
		"--cert="+certFile,
		"--key="+keyFile,
		"-n", "default",
		"--dry-run=client",
		"-o", "yaml",
	)
	secretYAML, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to create TLS secret manifest for %s: %v\nOutput:\n%s", secretName, err, string(secretYAML))
	}
	MustKubectlApplyStdin(ctx, kubeContext, string(secretYAML))

	log.Printf("Created TLS secret %s for hostname %s", secretName, hostname)
}

// CreateBasicAuthCombinedSecret creates a Secret usable by BOTH emitters:
// - kgateway: expects htpasswd content under key "auth"
// - agentgateway: expects htpasswd content under key ".htaccess"
func CreateBasicAuthCombinedSecret(ctx context.Context, kubeContext, secretName string) {
	if _, err := Kubectl(ctx, kubeContext, "get", "secret", secretName, "-n", "default"); err == nil {
		log.Printf("Basic auth combined secret %s already exists, skipping creation", secretName)
		return
	}

	// Same htpasswd line for both keys. (This is base64 for: user:{SHA}...)
	htpasswdB64 := "dXNlcjp7U0hBfVc2cGg1TW01UHo4R2dpVUxiUGd6RzM3bWo5Zz0="

	secretYAML := `
apiVersion: v1
kind: Secret
metadata:
  name: ` + secretName + `
  namespace: default
type: Opaque
data:
  auth: ` + htpasswdB64 + `
  .htaccess: ` + htpasswdB64 + `
`
	MustKubectlApplyStdin(ctx, kubeContext, secretYAML)
	log.Printf("Created basic auth combined secret %s", secretName)
}

func ApplyExternalAuthService(ctx context.Context, kubeContext string) {
	// Deploy a simple Go HTTP server that acts as an external auth provider
	// The server checks for Authorization: Bearer test-token header or ?auth=valid query param
	// Returns 200 OK with X-Auth-Token and X-User-ID headers on success, 401 on failure
	y := `
apiVersion: v1
kind: Service
metadata:
  name: auth-service
  namespace: default
spec:
  selector:
    app: auth-service
  ports:
  - name: http
    port: 8080
    targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: auth-service
  template:
    metadata:
      labels:
        app: auth-service
    spec:
      containers:
      - name: auth-service
        image: golang:1.21-alpine
        command:
        - sh
        - -c
        - |
          cat > /tmp/auth-server.go << 'EOF'
          package main
          import (
            "fmt"
            "log"
            "net/http"
            "os"
          )
          func main() {
            http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
              // Check for Authorization header or auth query parameter
              authHeader := r.Header.Get("Authorization")
              authQuery := r.URL.Query().Get("auth")
              
              // Log incoming request for debugging (skip health probe requests)
              if authQuery != "valid" {
                log.Printf("Auth service received request: Method=%s Path=%s Host=%s Headers=%v Query=%v", 
                  r.Method, r.URL.Path, r.Host, r.Header, r.URL.Query())
              }
              
              // Ensure we don't redirect - return direct response
              
              isValid := false
              if authHeader == "Bearer test-token" {
                isValid = true
              } else if authQuery == "valid" {
                isValid = true
              }
              
              // Set headers before writing status to avoid any redirect behavior
              if isValid {
                // Return 200 OK with response headers
                w.Header().Set("X-Auth-Token", "test-token")
                w.Header().Set("X-User-ID", "test-user")
                w.Header().Set("Content-Type", "text/plain")
                w.WriteHeader(http.StatusOK)
                fmt.Fprintf(w, "OK")
              } else {
                // Return 401 Unauthorized - must write status before body
                w.Header().Set("Content-Type", "text/plain")
                w.WriteHeader(http.StatusUnauthorized)
                fmt.Fprintf(w, "Unauthorized")
              }
            })
            log.Println("Starting auth server on 0.0.0.0:8080")
            if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
              log.Fatal(err)
              os.Exit(1)
            }
          }
          EOF
          go run /tmp/auth-server.go
        ports:
        - containerPort: 8080
        readinessProbe:
          httpGet:
            path: /?auth=valid
            port: 8080
          initialDelaySeconds: 2
          periodSeconds: 2
`

	log.Printf("Deploying external auth service")
	MustKubectlApplyStdin(ctx, kubeContext, y)
	MustKubectl(ctx, kubeContext, "-n", "default", "rollout", "status", "deploy/auth-service", "--timeout=5m")
}
