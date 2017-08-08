package prometheus

import (
	"os"
	"text/template"
	"bytes"
	"github.com/spf13/afero"
	"strings"
)

func GetGlobalConfig() string {
	data := getGlobalConfigData()
	config := `
global:`
	for key, values := range data {
		if len(values[""]) > 0 {
			config += "\n  " + key + ": " + values[""]
		} else {
			config += "\n  " + key + ":"
			for subKey, value := range values {
				config += "\n    " + subKey + ": " + value
			}
		}
	}
	return config
}

func GetScrapeConfig(scrapes map[string]Scrape) string {
	config := getScrapeConfigFromMap(scrapes) + getScrapeConfigFromSecrets()
	if len(config) > 0 {
		if !strings.HasPrefix(config, "\n") {
			config = "\n" + config
		}
		config = `
scrape_configs:` + config
	}
	return config
}

func WriteConfig(scrapes map[string]Scrape, alerts map[string]Alert) {
	FS.MkdirAll("/etc/prometheus", 0755)
	gc := GetGlobalConfig()
	sc := GetScrapeConfig(scrapes)
	ruleFiles := ""
	if len(alerts) > 0 {
		LogPrintf("Writing to alert.rules")
		ruleFiles = "\nrule_files:\n  - 'alert.rules'\n"
		afero.WriteFile(FS, "/etc/prometheus/alert.rules", []byte(GetAlertConfig(alerts)), 0644)
	}
	config := gc + "\n" + sc + ruleFiles
	LogPrintf("Writing to prometheus.yml")
	afero.WriteFile(FS, "/etc/prometheus/prometheus.yml", []byte(config), 0644)
}

func getGlobalConfigData() map[string]map[string]string {
	data := map[string]map[string]string{}
	for _, e := range os.Environ() {
		if key, value := getArgFromEnv(e, "GLOBAL"); len(key) > 0 {
			realKey := key
			subKey := ""
			if strings.Contains(key, "-") {
				keys := strings.Split(key, "-")
				realKey = keys[0]
				subKey = keys[1]
			}
			if _, ok := data[realKey]; !ok {
				data[realKey] = map[string]string{}
			}
			subData := data[realKey]
			subData[subKey] = value
		}
	}
	return data
}

func getScrapeConfigFromSecrets() string {
	config := ""
	if files, err := afero.ReadDir(FS, "/run/secrets/"); err == nil {
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "scrape_") {
				continue
			}
			if content, err := afero.ReadFile(FS, "/run/secrets/" + file.Name()); err == nil {
				config += string(content)
				if !strings.HasSuffix(config, "\n") {
					config += "\n"
				}
			}
		}
	}
	return config
}

func getScrapeConfigFromMap(scrapes map[string]Scrape) string {
	if len(scrapes) != 0 {
		templateString := `{{range .}}
  - job_name: "{{.ServiceName}}"
{{- if .ScrapeType}}
    {{.ScrapeType}}:
      - targets:
        - {{.ServiceName}}:{{- .ScrapePort}}
{{- else}}
    dns_sd_configs:
      - names: ["tasks.{{.ServiceName}}"]
        type: A
        port: {{.ScrapePort -}}{{end -}}
{{end}}
`
		tmpl, _ := template.New("").Parse(templateString)
		var b bytes.Buffer
		tmpl.Execute(&b, scrapes)
		return b.String()

	}
	return ""
}