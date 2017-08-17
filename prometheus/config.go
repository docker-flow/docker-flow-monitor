package prometheus

import (
	"bytes"
	"github.com/spf13/afero"
	"os"
	"strings"
	"text/template"
)

// WriteConfig creates Prometheus configuration (`/etc/prometheus/prometheus.yml`) and rules (`/etc/prometheus/alert.rules`) files.
func WriteConfig(scrapes map[string]Scrape, alerts map[string]Alert) {
	FS.MkdirAll("/etc/prometheus", 0755)
	gc := getGlobalConfig()
	sc := getScrapeConfig(scrapes)
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

func getGlobalConfig() string {
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

func getScrapeConfig(scrapes map[string]Scrape) string {
	config := getScrapeConfigFromMap(scrapes) + getScrapeConfigFromDir()
	if len(config) > 0 {
		if !strings.HasPrefix(config, "\n") {
			config = "\n" + config
		}
		config = `
scrape_configs:` + config
	}
	return config
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

func getScrapeConfigFromDir() string {
	config := ""
	dir := "/run/secrets/"
	if len(os.Getenv("CONFIGS_DIR")) > 0 {
		dir = os.Getenv("CONFIGS_DIR")
	}
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	if files, err := afero.ReadDir(FS, dir); err == nil {
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "scrape_") {
				continue
			}
			if content, err := afero.ReadFile(FS, dir+file.Name()); err == nil {
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
