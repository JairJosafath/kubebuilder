package webpage

import (
	"fmt"
	"html/template"
	"strings"
)

// Content is what the webpage shows.
type Content struct {
	// Title is the page heading, normally the Superpod name.
	Title string
	// Ability is the super ability text.
	Ability string
	// Emoji are shown in order below the ability. The page has no emoji grid
	// when Emoji is empty.
	Emoji []string
}

// maxColumns places one emoji alone, two side by side, and four in a 2×2 square.
const maxColumns = 2

// html/template escapes each value for where it appears, so abilities and
// emoji are always shown as text, even when they contain markup.
var page = template.Must(template.New("index.html").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Superpod</title>
</head>
<body>
  <h1>{{.Title}}</h1>
  <p>My super ability is: {{.Ability}}</p>
{{- with .Emoji}}
  <div aria-label="Ability emoji" style="display: inline-grid; grid-template-columns: repeat({{$.Columns}}, auto); gap: 0.5rem; font-size: 3rem">
{{- range .}}<span>{{.}}</span>{{end -}}
  </div>
{{- end}}
</body>
</html>
`))

// Render returns the webpage's index.html.
func Render(c Content) (string, error) {
	var b strings.Builder
	view := struct {
		Content
		Columns int
	}{c, min(maxColumns, len(c.Emoji))}
	if err := page.Execute(&b, view); err != nil {
		return "", fmt.Errorf("render webpage: %w", err)
	}
	return b.String(), nil
}
