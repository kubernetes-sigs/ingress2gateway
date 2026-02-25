package i2gw

import (
	"fmt"
	"go/ast"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	KubebuilderMaxItemsMarker = "kubebuilder:validation:MaxItems"
	MaxItemsPrefix            = "MaxItems="
)

// ValidateMaxItems validate fields presnet in the gateway resources object
func ValidateMaxItems(gwResources *GatewayResources) field.ErrorList {
	var errs field.ErrorList
	maxItemsMap, err := loadGatewayAPITypes()
	if err != nil {
		return append(errs, field.InternalError(nil,
			fmt.Errorf("unable to fetch kubebuilder max items : %v", err)))
	}

	for _, gw := range gwResources.Gateways {

		if reflect.ValueOf(gw).IsZero() {
			continue
		}

		v := reflect.ValueOf(gw)
		if v.Kind() == reflect.Ptr {
			//Get the struct
			v = v.Elem()
		}

		spec := v.FieldByName("Spec")
		if !spec.IsValid() || spec.IsZero() {
			errs = append(errs, field.Required(
				field.NewPath("spec"),
				"spec field missing or empty in gateway resource",
			))
			return errs
		}

		gatewayErrs := validateSpecFields(spec, maxItemsMap, field.NewPath("spec"))
		errs = append(errs, gatewayErrs...)

	}
	return errs
}

// validateSpecFields check fields in spec that are slices and whether exceeded max items limit
func validateSpecFields(spec reflect.Value, maxItemsMap map[string]int, path *field.Path) field.ErrorList {
	var errs field.ErrorList

	t := spec.Type()

	for i := 0; i < spec.NumField(); i++ {
		fieldVal := spec.Field(i)
		fieldType := t.Field(i)
		fieldName := fieldType.Name

		if fieldVal.Kind() == reflect.Slice && fieldVal.Len() > 0 {
			if maxItems, exists := maxItemsMap[fieldName]; exists {
				if fieldVal.Len() > maxItems {
					errs = append(errs, field.TooMany(
						path.Child(strings.ToLower(fieldName)),
						fieldVal.Len(),
						maxItems,
					))
				}
			}
		}
	}

	return errs
}

// loadGatewayAPITypes scans the Gateway API packages and extracts
// the `+kubebuilder:validation:MaxItems` values defined in struct field comments
func loadGatewayAPITypes() (map[string]int, error) {

	cfg := &packages.Config{
		Mode: packages.NeedFiles | packages.NeedSyntax,
	}

	pkgs, err := packages.Load(cfg,
		"sigs.k8s.io/gateway-api/apis/v1",
		"sigs.k8s.io/gateway-api/apis/v1alpha2",
		"sigs.k8s.io/gateway-api/apis/v1beta1",
		"sigs.k8s.io/gateway-api/apis/v1alpha3",
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load gateway-api package: %w", err)
	}

	maxItemsMap := make(map[string]int)

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Field:
					if x.Doc != nil {
						for _, comment := range x.Doc.List {
							if strings.Contains(comment.Text, KubebuilderMaxItemsMarker) {
								fieldName := x.Names[0].Name
								if strings.Contains(comment.Text, MaxItemsPrefix) {
									parts := strings.Split(comment.Text, MaxItemsPrefix)
									if len(parts) > 1 {
										valueStr := strings.Split(parts[1], " ")[0]
										if maxItems, err := strconv.Atoi(valueStr); err == nil {
											maxItemsMap[fieldName] = maxItems
										}

									}
								}
							}
						}
					}
				}
				return true
			})
		}
	}

	return maxItemsMap, nil
}
