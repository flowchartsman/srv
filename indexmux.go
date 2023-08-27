package srv

import (
	"html/template"
	"net/http"
)

type indexMux struct {
	*http.ServeMux
	Service string
	Links   []*muxLink
}

func newIndexMux() *indexMux {
	im := &indexMux{
		ServeMux: http.NewServeMux(),
	}
	im.ServeMux.HandleFunc("/", im.list)
	return im
}

func (im *indexMux) addLink(href, description string) {
	im.Links = append(im.Links, &muxLink{
		Href:        href,
		Description: description,
	})
}

func (im *indexMux) Handle(pattern string, handler http.Handler, description string) {
	im.addLink(pattern, description)
	im.ServeMux.Handle(pattern, handler)
}

func (im *indexMux) HandleFunc(pattern string, handler http.HandlerFunc, description string) {
	im.addLink(pattern, description)
	im.ServeMux.HandleFunc(pattern, handler)
}

func (im *indexMux) list(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	listTmpl.Execute(w, im)
}

type muxLink struct {
	Href        string
	Description string
}

var listTmpl = template.Must(template.New("list").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>{{.Service }} Instrumentation Links</title>
<body>
<table>
{{ range .Links -}}
<tr><td><a href="{{.Href}}"><pre>{{.Href}}</pre></a></td><td>{{.Description}}</td></tr>
{{ end -}}
</table>
</body>
</html>
`))
