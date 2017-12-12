package prometheus

import (
	"bytes"
	"text/template"
)

// GetAlertConfig returns Prometheus configuration snippet related to alerts.
func GetAlertConfig(alerts map[string]Alert) string {
	templateString := `groups:
- name: alert.rules
  rules:
  {{- range . }}
  - alert: {{ .AlertNameFormatted }}
    expr: {{ .AlertIf }}
    {{- if .AlertFor }}
    for: {{ .AlertFor }}
    {{- end }}
    {{- if .AlertLabels }}
    labels:
    {{- range $key, $value := .AlertLabels }}
      {{ $key }}: {{ $value }}
    {{- end}}
    {{- end }}
    {{- if .AlertAnnotations }}
    annotations:
    {{- range $key, $value := .AlertAnnotations }}
      {{ $key }}: "{{ $value }}"
    {{- end}}
    {{- end }}
  {{- end }}`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, alerts)
	return b.String()
}
