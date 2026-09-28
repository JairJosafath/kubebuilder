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
		// Two columns: two emoji sit side by side and four form a 2×2 square.
		cells := make([]string, len(emojis))
		for i, emoji := range emojis {
			cells[i] = "<span>" + html.EscapeString(emoji) + "</span>"
		}
		emojiHTML = fmt.Sprintf("\n  <div aria-label=\"Ability emoji\" style=\"display: inline-grid; "+
			"grid-template-columns: repeat(%d, auto); gap: 0.5rem; font-size: 3rem\">%s</div>",
			min(2, len(emojis)), strings.Join(cells, ""))
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
