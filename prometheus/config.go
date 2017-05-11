package prometheus

import (
	"os"
	"text/template"
	"bytes"
	"github.com/spf13/afero"
)

func GetGlobalConfig() string {
	config := `
global:`
	for _, e := range os.Environ() {
		if key, value := getArgFromEnv(e, "GLOBAL"); len(key) > 0 {
			config = config + "\n  " + key + ": " + value
		}
	}
	return config
}

func GetScrapeConfig(scrapes map[string]Scrape) string {
	if len(scrapes) == 0 {
		return ""
	}
	templateString := `
scrape_configs:{{range .}}
  - job_name: "{{.ServiceName}}"
    dns_sd_configs:
      - names: ["tasks.{{.ServiceName}}"]
        type: A
        port: {{.ScrapePort}}{{end}}
`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, scrapes)
	return b.String()
}

func WriteConfig(scrapes map[string]Scrape, alerts map[string]Alert) {
	FS.MkdirAll("/etc/prometheus", 0755)
	gc := GetGlobalConfig()
	sc := GetScrapeConfig(scrapes)
	ruleFiles := ""
	if len(alerts) > 0 {
		logPrintf("Writing to alert.rules")
		ruleFiles = "\nrule_files:\n  - 'alert.rules'\n"
		afero.WriteFile(FS, "/etc/prometheus/alert.rules", []byte(GetAlertConfig(alerts)), 0644)
	}
	config := gc + "\n" + sc + ruleFiles
	logPrintf("Writing to prometheus.yml")
	afero.WriteFile(FS, "/etc/prometheus/prometheus.yml", []byte(config), 0644)
}
