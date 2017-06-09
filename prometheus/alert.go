package prometheus

import (
	"text/template"
	"bytes"
)

func GetAlertConfig(alerts map[string]Alert) string {
	// TODO: Add ANNOTATIONS
	templateString := `{{range .}}
ALERT {{.AlertNameFormatted}}
  IF {{.AlertIf}}{{if .AlertFor}}
  FOR {{.AlertFor}}{{end}}
  {{- if .AlertLabels}}
  LABELS {
    {{- range $key, $value := .AlertLabels}}
    {{$key}} = "{{$value}}",
    {{- end}}
  }
  {{- end}}
  {{- if .AlertAnnotations}}
  ANNOTATIONS {
    {{- range $key, $value := .AlertAnnotations}}
    {{$key}} = "{{$value}}",
    {{- end}}
  }
  {{- end}}
{{end}}`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, alerts)
	return b.String()
}
