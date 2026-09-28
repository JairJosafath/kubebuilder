package resources

import (
	networkingv1 "k8s.io/api/networking/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// NewIngress routes the requested hostname to the nginx Service.
// An installed Ingress controller and DNS configuration are still required.
func NewIngress(sp *superv1.Superpod) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metadata(sp),
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: sp.Spec.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: resourceName(sp),
									Port: networkingv1.ServiceBackendPort{Name: httpPortName},
								},
							},
						}},
					},
				},
			}},
		},
	}
	if sp.Spec.IngressClassName != "" {
		className := sp.Spec.IngressClassName
		ingress.Spec.IngressClassName = &className
	}
	return ingress
}
