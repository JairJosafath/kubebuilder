package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// The Service, Ingress, and readiness probe refer to the same named port.
const httpPortName = "http"

// NewPod builds one nginx container serving the ConfigMap's index.html.
func NewPod(sp *superv1.Superpod) *corev1.Pod {
	automountToken := false
	return &corev1.Pod{
		ObjectMeta: metadata(sp),
		Spec: corev1.PodSpec{
			ServiceAccountName:           resourceName(sp),
			AutomountServiceAccountToken: &automountToken,
			Containers: []corev1.Container{{
				Name:            "nginx",
				Image:           "nginx:stable-alpine",
				ImagePullPolicy: corev1.PullAlways,
				Ports: []corev1.ContainerPort{{
					Name:          httpPortName,
					ContainerPort: 80,
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "html",
					MountPath: "/usr/share/nginx/html",
					ReadOnly:  true,
					// Mount the directory without subPath so ConfigMap updates
					// can reach the running container.
				}},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/",
							Port: intstr.FromString(httpPortName),
						},
					},
				},
			}},
			Volumes: []corev1.Volume{{
				Name: "html",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: resourceName(sp),
						},
					},
				},
			}},
		},
	}
}
