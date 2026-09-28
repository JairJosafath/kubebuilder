package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// NewService gives the nginx Pod an internal address for the Ingress to use.
func NewService(sp *superv1.Superpod) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metadata(sp),
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: podSelector(sp),
			Ports: []corev1.ServicePort{{
				Name:       httpPortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromString(httpPortName),
			}},
		},
	}
}
