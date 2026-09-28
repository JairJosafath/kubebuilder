package webpage_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/jairjosafath/operator/internal/webpage"
)

const goldenHead = `<!doctype html>
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

// TestRenderMatchesGolden pins the page bytes. A byte change rewrites the
// ConfigMap of every running Superpod, so change the page only on purpose.
func TestRenderMatchesGolden(t *testing.T) {
	for _, test := range []struct {
		name  string
		emoji []string
		want  string
	}{
		{"plain", nil, goldenHead + "</body>\n</html>\n"},
		{"two emoji", []string{"🦅", "🪽"}, goldenHead + `  <div aria-label="Ability emoji" style="display: inline-grid; ` +
			`grid-template-columns: repeat(2, auto); gap: 0.5rem; font-size: 3rem"><span>🦅</span><span>🪽</span></div>
</body>
</html>
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := webpage.Render(webpage.Content{
				Title: "superpod-example", Ability: `<b>Flying</b> & "more"`, Emoji: test.emoji,
			})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Render() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderEscapesAllText(t *testing.T) {
	got, err := webpage.Render(webpage.Content{
		Title:   "<i>title</i>",
		Ability: "<script>ability</script>",
		Emoji:   []string{"🦅", "<script>emoji</script>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<script>") || strings.Contains(got, "<i>") || !strings.Contains(got, "🦅") ||
		!strings.Contains(got, "&lt;script&gt;emoji&lt;/script&gt;") {
		t.Fatalf("the page must show Unicode and escape all text:\n%s", got)
	}
}

func TestRenderLayout(t *testing.T) {
	for _, test := range []struct {
		emoji   []string
		columns string
	}{
		{[]string{"🦅"}, "repeat(1, auto)"},
		{[]string{"🦅", "🪽"}, "repeat(2, auto)"},
		{[]string{"🦅", "🪽", "✈️", "🚀"}, "repeat(2, auto)"},
	} {
		got, err := webpage.Render(webpage.Content{Title: "t", Ability: "a", Emoji: test.emoji})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, test.columns) || strings.Count(got, "<span>") != len(test.emoji) {
			t.Fatalf("%d emoji need %s with one cell each:\n%s", len(test.emoji), test.columns, got)
		}
	}
}
