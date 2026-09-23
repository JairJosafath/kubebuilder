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

package resources

import (
	"fmt"
	"html"
	"strings"

	corev1 "k8s.io/api/core/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// NewConfigMap builds the webpage. Escape user input so an ability is displayed
// as text, even if it contains HTML tags.
func NewConfigMap(sp *superv1.Superpod, emojis ...string) *corev1.ConfigMap {
	emojiHTML := ""
	if len(emojis) > 0 {
		emojiHTML = "\n  <p aria-label=\"Ability emoji\" style=\"font-size: 3rem\">" +
			html.EscapeString(strings.Join(emojis, " ")) + "</p>"
	}
	page := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Superpod</title>
</head>
<body>
  <h1>%s</h1>
  <p>My super ability is: %s</p>%s
</body>
</html>
`, html.EscapeString(sp.Name), html.EscapeString(sp.Spec.SuperAbility), emojiHTML)

	return &corev1.ConfigMap{
		ObjectMeta: metadata(sp),
		Data: map[string]string{
			"index.html": page,
		},
	}
}
