package translator

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const (
	jsonOutput = "json"
	yamlOutput = "yaml"
)

const (
	LocalMode  = "local"
	RemoteMode = "remote"
)

type GWAPIResourceType string

var (
	AllGWAPIResourceType       GWAPIResourceType = "all"
	HTTPRouteGWAPIResourceType GWAPIResourceType = "httproute"
	GatewayGWAPIResourceType   GWAPIResourceType = "gateway"
)

func validResourceTypes() []GWAPIResourceType {
	return []GWAPIResourceType{
		AllGWAPIResourceType,
		HTTPRouteGWAPIResourceType,
		GatewayGWAPIResourceType,
	}
}

func isValidResourceType(outType GWAPIResourceType) bool {
	for _, vType := range validResourceTypes() {
		if outType == vType {
			return true
		}
	}
	return false
}

func GetValidResourceTypesStr() string {
	return fmt.Sprintf("Valid types are %v.", validResourceTypes())
}

func nameFromHost(host string) string {
	// replace all special chars with -
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	step1 := reg.ReplaceAllString(host, "-")
	// remove all - at start of string
	reg2 := regexp.MustCompile("^[^a-zA-Z0-9]+")
	step2 := reg2.ReplaceAllString(step1, "")
	// if nothing left, return "all-hosts"
	if len(host) == 0 {
		return "all-hosts"
	}
	return step2
}

func getInputBytes(inFile string) ([]byte, error) {

	// Get input from stdin
	if inFile == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		var input string
		for {
			if !scanner.Scan() {
				break
			}
			input += scanner.Text() + "\n"
		}
		return []byte(input), nil
	}
	// Get input from file
	return os.ReadFile(inFile)
}

// kubernetesYAMLToResources converts a Kubernetes YAML string into Ingresses
func kubernetesYAMLToResources(str string) ([]networkingv1.Ingress, error) {
	resources := []networkingv1.Ingress{}
	yamls := strings.Split(str, "\n---")
	ingressScheme := runtime.NewScheme()
	err := networkingv1.AddToScheme(ingressScheme)
	if err != nil {
		return nil, err
	}

	for _, y := range yamls {
		if strings.TrimSpace(y) == "" {
			continue
		}
		var obj map[string]interface{}
		err := yaml.Unmarshal([]byte(y), &obj)
		if err != nil {
			return nil, err
		}
		un := unstructured.Unstructured{Object: obj}
		gvk := un.GroupVersionKind()
		name, namespace := un.GetName(), un.GetNamespace()
		kobj, err := ingressScheme.New(gvk)
		if err != nil {
			return nil, err
		}
		err = ingressScheme.Convert(&un, kobj, nil)
		if err != nil {
			return nil, err
		}

		objType := reflect.TypeOf(kobj)
		if objType.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("expected pointer type, but got %s", objType.Kind().String())
		}
		kobjVal := reflect.ValueOf(kobj).Elem()
		spec := kobjVal.FieldByName("Spec")

		if gvk.Kind == "Ingress" {
			typedSpec := spec.Interface()
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   namespace,
					Annotations: un.GetAnnotations(),
					Labels:      un.GetLabels(),
				},
				Spec: typedSpec.(networkingv1.IngressSpec),
			}
			resources = append(resources, *ingress)
		}
	}

	return resources, nil
}
