package prometheus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

// WriteConfig creates Prometheus configuration at configPath and writes alerts into /etc/prometheus/alert.rules
func WriteConfig(configPath string, scrapes map[string]Scrape,
	alerts map[string]Alert, nodeLabels map[string]map[string]string) {
	c := &Config{}
	fileSDDir := "/etc/prometheus/file_sd"
	alertRulesPath := "/etc/prometheus/alert.rules"

	configDir := filepath.Dir(configPath)
	FS.MkdirAll(configDir, 0755)
	FS.MkdirAll(fileSDDir, 0755)
	c.InsertScrapes(scrapes)

	if len(alerts) > 0 {
		logPrintf("Writing to alert.rules")
		afero.WriteFile(FS, alertRulesPath, []byte(GetAlertConfig(alerts)), 0644)
		c.RuleFiles = []string{"alert.rules"}
	}

	alertmanagerURL := os.Getenv("ARG_ALERTMANAGER_URL")
	if len(alertmanagerURL) != 0 {
		if err := c.InsertAlertManagerURL(alertmanagerURL); err != nil {
			logPrintf("Unable to insert alertmanager url %s into prometheus config", alertmanagerURL)
		}
	}
	c.CreateFileStaticConfig(scrapes, nodeLabels, fileSDDir)

	for _, e := range os.Environ() {
		envSplit := strings.SplitN(e, "=", 2)
		if len(envSplit) != 2 {
			continue
		}
		key, value := envSplit[0], envSplit[1]

		if strings.HasPrefix(key, "GLOBAL_") ||
			strings.HasPrefix(key, "ALERTING_") ||
			strings.HasPrefix(key, "SCRAPE_CONFIGS_") ||
			strings.HasPrefix(key, "REMOTE_WRITE_") ||
			strings.HasPrefix(key, "REMOTE_READ_") {
			if err := c.InsertEnv(key, value); err != nil {
				logPrintf("Unable to insert %s into prometheus config", e)
			}
		}
	}

	logPrintf("Writing to prometheus.yml")
	configYAML, _ := yaml.Marshal(c)
	afero.WriteFile(FS, configPath, configYAML, 0644)

}

// InsertEnv inserts envKey/envValue into config
func (c *Config) InsertEnv(envKey string, envValue string) error {
	envKey, envValue = convertToV2Env(envKey, envValue)
	envKey = strings.ToLower(envKey)
	obj := reflect.ValueOf(c)
	location := strings.Split(envKey, "__")
	return insertWithLocation(obj, location, envValue, 0)
}

// InsertAlertManagerURL inserts alert into config
func (c *Config) InsertAlertManagerURL(alertURL string) error {
	url, err := url.Parse(alertURL)
	if err != nil {
		return fmt.Errorf("Unable to parse url %s", alertURL)
	}

	amc := &AlertmanagerConfig{
		Scheme: url.Scheme,
		ServiceDiscoveryConfig: ServiceDiscoveryConfig{
			StaticConfigs: []*TargetGroup{{
				Targets: []string{url.Host},
			}},
		},
	}

	c.AlertingConfig.AlertmanagerConfigs = append(c.AlertingConfig.AlertmanagerConfigs, amc)
	return nil
}

// InsertScrapes inserts scrapes into config
func (c *Config) InsertScrapes(scrapes map[string]Scrape) {

	for _, s := range scrapes {
		var newScrape *ScrapeConfig
		metricsPath := s.MetricsPath
		if len(metricsPath) == 0 {
			metricsPath = "/metrics"
		}
		if s.NodeInfo != nil && len(s.NodeInfo) > 0 {
			continue
		}
		if s.ScrapeType == "static_configs" {
			newScrape = &ScrapeConfig{
				ServiceDiscoveryConfig: ServiceDiscoveryConfig{
					StaticConfigs: []*TargetGroup{{
						Targets: []string{fmt.Sprintf("%s:%d", s.ServiceName, s.ScrapePort)},
					}},
				},
			}
		} else {
			newScrape = &ScrapeConfig{
				ServiceDiscoveryConfig: ServiceDiscoveryConfig{
					DNSSDConfigs: []*DNSSDConfig{{
						Names: []string{fmt.Sprintf("tasks.%s", s.ServiceName)},
						Port:  s.ScrapePort,
						Type:  "A",
					}},
				},
			}
		}
		newScrape.JobName = s.ServiceName
		newScrape.MetricsPath = metricsPath
		newScrape.ScrapeInterval = s.ScrapeInterval
		newScrape.ScrapeTimeout = s.ScrapeTimeout
		c.ScrapeConfigs = append(c.ScrapeConfigs, newScrape)
	}
}

// InsertScrapesFromDir inserts scrapes from directory
func (c *Config) InsertScrapesFromDir(dir string) {
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	if files, err := afero.ReadDir(FS, dir); err == nil {
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "scrape_") {
				continue
			}
			if content, err := afero.ReadFile(FS, dir+file.Name()); err == nil {
				sc := []*ScrapeConfig{}

				// Trim for backwards compatibility
				content = normalizeScrapeFile(content)
				err := yaml.Unmarshal(content, &sc)
				if err != nil {
					continue
				}
				c.ScrapeConfigs = append(c.ScrapeConfigs, sc[0])
			}
		}

	}

}

// CreateFileStaticConfig creates static config files
func (c *Config) CreateFileStaticConfig(scrapes map[string]Scrape, nodeLabels map[string]map[string]string, fileSDDir string) {

	staticFiles := map[string]struct{}{}
	for _, s := range scrapes {
		fsc := FileStaticConfig{}
		if s.NodeInfo == nil {
			continue
		}
		for n := range s.NodeInfo {
			tg := TargetGroup{}
			tg.Targets = []string{fmt.Sprintf("%s:%d", n.Addr, s.ScrapePort)}
			tg.Labels = map[string]string{}
			if s.ScrapeLabels != nil {
				for k, v := range *s.ScrapeLabels {
					tg.Labels[k] = v
				}
			}
			tg.Labels["node"] = n.Name
			tg.Labels["service"] = s.ServiceName

			// If there is a node id add nodeLabels[n.ID] to service
			if labels, ok := nodeLabels[n.ID]; len(n.ID) > 0 && ok && labels != nil {
				for k, v := range labels {
					tg.Labels[k] = v
				}
			}
			fsc = append(fsc, &tg)
		}

		if len(fsc) == 0 {
			continue
		}

		fscBytes, err := json.Marshal(fsc)
		if err != nil {
			continue
		}
		filePath := fmt.Sprintf("%s/%s.json", fileSDDir, s.ServiceName)
		afero.WriteFile(FS, filePath, fscBytes, 0644)
		newScrape := &ScrapeConfig{
			ServiceDiscoveryConfig: ServiceDiscoveryConfig{
				FileSDConfigs: []*SDConfig{{
					Files: []string{filePath},
				}},
			},
			JobName: s.ServiceName,
		}
		c.ScrapeConfigs = append(c.ScrapeConfigs, newScrape)
		staticFiles[filePath] = struct{}{}
	}

	// Remove scrapes that are not in fileStaticServices
	currentStaticFiles, err := afero.Glob(FS, fmt.Sprintf("%s/*.json", fileSDDir))
	if err != nil {
		return
	}
	for _, file := range currentStaticFiles {
		if _, ok := staticFiles[file]; !ok {
			FS.Remove(file)
		}
	}
}

func normalizeScrapeFile(content []byte) []byte {
	spaceCnt := 0
	for i, c := range content {
		if c != ' ' {
			spaceCnt = i
			break
		}
	}
	bf := new(bytes.Buffer)
	bf.WriteByte('\n')
	for i := 0; i < spaceCnt; i++ {
		bf.WriteByte(' ')
	}
	content = bytes.TrimLeft(content, " ")
	return bytes.Replace(content, bf.Bytes(), []byte{'\n'}, -1)
}

func convertToV2Env(envKey string, envValue string) (string, string) {
	if strings.Contains(envKey, "__") {
		return envKey, envValue
	}

	envKey = strings.Replace(envKey, "GLOBAL_", "GLOBAL__", 1)
	envKey = strings.Replace(envKey, "REMOTE_WRITE_", "REMOTE_WRITE_1__", 1)
	envKey = strings.Replace(envKey, "REMOTE_READ_", "REMOTE_READ_1__", 1)

	if strings.HasPrefix(envKey, "GLOBAL__EXTERNAL_LABELS") {
		envSplit := strings.Split(envKey, "-")
		if len(envSplit) == 2 {
			envKey = envSplit[0]
			envValue = fmt.Sprintf("%s=%s", strings.ToLower(envSplit[1]), envValue)
		}
	}
	return envKey, envValue
}

var sliceRegex = regexp.MustCompile("^(.+)_(\\d+)$")

func insertWithLocation(obj reflect.Value, location []string, value string, index int) error {
	switch obj.Kind() {
	case reflect.Ptr:
		return insertWithLocationPtr(obj, location, value, index)
	case reflect.Struct:
		return insertWithLocationStruct(obj, location, value, index)
	case reflect.Slice:
		return insertWithLocationSlice(obj, location, value, index)
	case reflect.Map:
		return insertWithLocationMap(obj, location, value, index)
	default:
		return insertWithLocationDefault(obj, location, value, index)
	}
}

func insertWithLocationDefault(obj reflect.Value, location []string, value string, index int) error {
	objI := obj.Interface()
	switch objI.(type) {
	case string:
		obj.SetString(value)
	case bool:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("Non bool value")
		}
		obj.Set(reflect.ValueOf(v))
	case int:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("Non int value")
		}
		obj.Set(reflect.ValueOf(v))
	case uint64:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("Non int value")
		}
		obj.Set(reflect.ValueOf(uint64(v)))
	case uint:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("Non int value")
		}
		obj.Set(reflect.ValueOf(uint(v)))
	}
	return nil
}

func insertWithLocationPtr(obj reflect.Value, location []string, value string, index int) error {
	if obj.IsNil() {
		newObj := reflect.New(obj.Type().Elem())
		obj.Set(newObj)
	}
	objElem := obj.Elem()
	return insertWithLocation(objElem, location, value, index)
}

func insertWithLocationStruct(obj reflect.Value, location []string, value string, index int) error {
	t := reflect.TypeOf(obj.Interface())
	if index >= len(location) {
		return fmt.Errorf("incorrect env location")
	}
	targetTag := location[index]
	for i := 0; i < t.NumField(); i++ {
		tField := t.Field(i)
		tags := tField.Tag.Get("yaml")
		tagsSplit := strings.Split(tags, ",")
		tag := tagsSplit[0]

		// struct or primitive
		if targetTag == tag {
			v := obj.Field(i)
			return insertWithLocation(v, location, value, index+1)
		}

		// slice
		rootTagR := sliceRegex.FindAllStringSubmatch(targetTag, 1)
		if len(rootTagR) == 1 && len(rootTagR[0]) == 3 {
			rootTag := rootTagR[0][1]
			if rootTag == tag {
				v := obj.Field(i)
				return insertWithLocation(v, location, value, index)
			}
		}

		// inline struct (always last element)
		if tagsSplit[len(tagsSplit)-1] == "inline" {
			v := obj.Field(i)
			err := insertWithLocation(v, location, value, index)
			if err != nil {
				continue
			}
		}
	}
	return fmt.Errorf("Unable to find tag: %s", targetTag)
}

func insertWithLocationSlice(obj reflect.Value, location []string, value string, index int) error {
	if index >= len(location) {
		return fmt.Errorf("Incorrect env location")
	}
	targetTag := location[index]
	sliceTag := sliceRegex.FindAllStringSubmatch(targetTag, 1)
	if len(sliceTag) == 0 || len(sliceTag[0]) != 3 {
		return fmt.Errorf("Array tag must be of the form: label_NUM")
	}
	indexValue, err := strconv.Atoi(sliceTag[0][2])
	if err != nil {
		return fmt.Errorf("Array tag must end with a number")
	}
	if obj.Len() < indexValue {
		newVP := reflect.New(obj.Type()).Elem()
		newVP.Set(reflect.MakeSlice(obj.Type(), indexValue, indexValue))
		reflect.Copy(newVP, obj)
		obj.Set(newVP)
	}
	return insertWithLocation(obj.Index(indexValue-1), location, value, index+1)
}

func insertWithLocationMap(obj reflect.Value, location []string, value string, index int) error {
	// All Maps are map[string]string or map[string][]string
	keyValue := strings.Split(value, "=")
	if len(keyValue) != 2 {
		return fmt.Errorf("Value for map must be of the form: key=value")
	}
	if obj.IsNil() {
		obj.Set(reflect.MakeMap(obj.Type()))
	}

	// handle map[string][]string
	key := keyValue[0]
	sliceTag := sliceRegex.FindAllStringSubmatch(key, 1)
	if len(sliceTag) == 1 && len(sliceTag[0]) == 3 {
		location = append(location, sliceTag[0][0])
		key = sliceTag[0][1]
	}

	newV := reflect.New(obj.Type().Elem()).Elem()

	if newV.Kind() == reflect.Slice {
		previousV := obj.MapIndex(reflect.ValueOf(key))
		if previousV.IsValid() {
			pLen := previousV.Len()
			newV.Set(reflect.MakeSlice(obj.Type().Elem(), pLen, pLen))
			reflect.Copy(newV, previousV)
		}
	}

	err := insertWithLocation(newV, location, keyValue[1], index)
	if err != nil {
		return err
	}
	obj.SetMapIndex(reflect.ValueOf(key), newV)
	return nil
}
