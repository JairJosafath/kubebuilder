package resources_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/resources"
	"github.com/jairjosafath/operator/internal/webpage"
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

func newConfigMap(t *testing.T, sp *superv1.Superpod, sel *webpage.Selection) *corev1.ConfigMap {
	t.Helper()
	cm, err := resources.NewConfigMap(sp, sel)
	if err != nil {
		t.Fatal(err)
	}
	return cm
}

func selectionOf(symbols ...string) *webpage.Selection {
	sel := &webpage.Selection{Key: "test-key"}
	for _, symbol := range symbols {
		sel.Matches = append(sel.Matches, emoji.Match{Emoji: symbol})
	}
	return sel
}

func TestResourceConnections(t *testing.T) {
	sp := exampleSuperpod()
	pod := resources.NewPod(sp)
	cm := newConfigMap(t, sp, nil)
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
	first := newConfigMap(t, sp, nil)
	pod := resources.NewPod(sp)
	if !reflect.DeepEqual(first, newConfigMap(t, sp, nil)) || !reflect.DeepEqual(sp, before) {
		t.Fatal("building resources must be repeatable without modifying the Superpod")
	}

	sp.Spec.SuperAbility = "<script>alert('Flying')</script> & invisibility"
	updated := newConfigMap(t, sp, nil)
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

// TestPageMatchesGoldenHTML pins the exact page bytes. Refactors must not change
// them: every byte change rewrites the ConfigMap of every running Superpod.
func TestPageMatchesGoldenHTML(t *testing.T) {
	const head = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Superpod</title>
</head>
<body>
  <h1>superpod-example</h1>
  <p>My super ability is: &lt;b&gt;Flying&lt;/b&gt; &amp; &#34;more&#34;</p>
`
	for _, test := range []struct {
		name string
		sel  *webpage.Selection
		want string
	}{
		{"plain", nil, head + "</body>\n</html>\n"},
		{"two emoji", selectionOf("🦅", "🪽"), head + `  <div aria-label="Ability emoji" style="display: inline-grid; ` +
			`grid-template-columns: repeat(2, auto); gap: 0.5rem; font-size: 3rem"><span>🦅</span><span>🪽</span></div>
</body>
</html>
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			sp := exampleSuperpod()
			sp.Spec.SuperAbility = `<b>Flying</b> & "more"`
			if got := newConfigMap(t, sp, test.sel).Data["index.html"]; got != test.want {
				t.Errorf("page changed:\n--- got\n%s\n--- want\n%s", got, test.want)
			}
		})
	}
}

func TestStoredSelectionRoundTrip(t *testing.T) {
	sel := &webpage.Selection{Key: "test-key", Matches: []emoji.Match{{Emoji: "🦅", Name: "eagle", Score: 1}}}
	cm := newConfigMap(t, exampleSuperpod(), sel)
	if diff := cmp.Diff(*sel, resources.StoredSelection(cm)); diff != "" {
		t.Errorf("StoredSelection() mismatch (-want +got):\n%s", diff)
	}
	if got := resources.StoredSelection(newConfigMap(t, exampleSuperpod(), nil)); got.Key != "" {
		t.Errorf("a plain page must not store a selection, got %+v", got)
	}
	if got := resources.StoredSelection(nil); got.Key != "" {
		t.Errorf("a missing ConfigMap must yield no selection, got %+v", got)
	}
	cm.Data["emoji-cache.json"] = "{corrupt"
	if got := resources.StoredSelection(cm); got.Key != "" {
		t.Errorf("a corrupt selection must be ignored, got %+v", got)
	}
}

func TestObjectKeyNamesEveryChild(t *testing.T) {
	sp := exampleSuperpod()
	key := resources.ObjectKey(sp)
	for _, child := range []metav1.Object{
		resources.NewServiceAccount(sp), newConfigMap(t, sp, nil), resources.NewPod(sp),
		resources.NewService(sp), resources.NewIngress(sp),
	} {
		if child.GetNamespace() != key.Namespace || child.GetName() != key.Name {
			t.Errorf("%T is named %s/%s, want %s", child, child.GetNamespace(), child.GetName(), key)
		}
	}
}
