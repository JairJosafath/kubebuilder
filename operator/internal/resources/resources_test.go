/*
Copyright 2026.

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

package resources_test

import (
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
)

func exampleSuperpod() *superv1.Superpod {
	return &superv1.Superpod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "superpod-example",
			Namespace: "default",
			UID:       "3aa43bd9-d1d3-42b8-99cf-b87f95c335c0",
		},
		Spec: superv1.SuperpodSpec{
			SuperAbility: "Flying",
			Host:         "superpod.example.test",
		},
	}
}

func TestResourceConnections(t *testing.T) {
	sp := exampleSuperpod()
	pod := resources.NewPod(sp)
	cm := resources.NewConfigMap(sp)
	sa := resources.NewServiceAccount(sp)
	service := resources.NewService(sp)
	ingress := resources.NewIngress(sp)

	volume := pod.Spec.Volumes[0]
	mount := pod.Spec.Containers[0].VolumeMounts[0]
	if volume.ConfigMap.Name != cm.Name || mount.Name != volume.Name || cm.Data["index.html"] == "" {
		t.Fatal("the nginx volume must connect to the HTML ConfigMap")
	}
	if !mount.ReadOnly || mount.SubPath != "" {
		t.Fatal("the HTML directory must be read-only and allow projected updates")
	}
	if pod.Spec.ServiceAccountName != sa.Name {
		t.Fatal("the Pod must use its dedicated ServiceAccount")
	}
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatal("the Ingress backend must use an internal Service")
	}
	selector := labels.SelectorFromSet(service.Spec.Selector)
	if !selector.Matches(labels.Set(pod.Labels)) {
		t.Fatal("the Service must select its Pod")
	}
	other := sp.DeepCopy()
	other.UID = "7a2d05b0-b1a7-4149-a5d7-fd6f07231f24"
	if selector.Matches(labels.Set(resources.NewPod(other).Labels)) {
		t.Fatal("the Service must not select a different Superpod's Pod")
	}
	backend := ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	if backend.Name != service.Name || backend.Port.Name != service.Spec.Ports[0].Name {
		t.Fatal("the Ingress must route to the Service port")
	}
	if service.Spec.Ports[0].TargetPort.StrVal != pod.Spec.Containers[0].Ports[0].Name {
		t.Fatal("the Service must route to the nginx container port")
	}
	if ingress.Spec.IngressClassName != nil {
		t.Fatal("an unspecified IngressClass must remain unset for cluster defaulting")
	}
	sp.Spec.IngressClassName = "example-class"
	if got := resources.NewIngress(sp).Spec.IngressClassName; got == nil || *got != sp.Spec.IngressClassName {
		t.Fatal("an explicit IngressClass must be preserved")
	}
}

func TestAbilityUpdateOnlyChangesHTML(t *testing.T) {
	sp := exampleSuperpod()
	before := sp.DeepCopy()
	first := resources.NewConfigMap(sp)
	pod := resources.NewPod(sp)
	if !reflect.DeepEqual(first, resources.NewConfigMap(sp)) || !reflect.DeepEqual(sp, before) {
		t.Fatal("building resources must be repeatable without modifying the Superpod")
	}

	sp.Spec.SuperAbility = "<script>alert('Flying')</script> & invisibility"
	updated := resources.NewConfigMap(sp)
	page := updated.Data["index.html"]
	if page == first.Data["index.html"] || strings.Contains(page, "<script>") ||
		!strings.Contains(page, "&lt;script&gt;") || !strings.Contains(page, "&amp; invisibility") {
		t.Fatal("changing an ability must update the page and escape HTML input")
	}
	if updated.Name != first.Name || !reflect.DeepEqual(pod, resources.NewPod(sp)) {
		t.Fatal("changing an ability must keep resource identity and the Pod definition stable")
	}
}

func TestLongSuperpodNameProducesValidServiceName(t *testing.T) {
	sp := exampleSuperpod()
	sp.Name = strings.Repeat("long-name.", 20) + "example"
	name := resources.NewService(sp).Name
	if problems := validation.IsDNS1035Label(name); len(problems) != 0 {
		t.Fatalf("resource name %q is invalid: %v", name, problems)
	}
}
