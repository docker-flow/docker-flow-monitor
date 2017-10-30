package prometheus

// Alert defines data used to create alert configuration snippet
type Alert struct {
	AlertAnnotations   map[string]string `json:"alertAnnotations,omitempty"`
	AlertFor           string            `json:"alertFor,omitempty"`
	AlertIf            string            `json:"alertIf,omitempty"`
	AlertLabels        map[string]string `json:"alertLabels,omitempty"`
	AlertName          string            `json:"alertName"`
	AlertNameFormatted string
	ServiceName        string `json:"serviceName"`
	Replicas           int    `json:"replicas"`
}

// Scrape defines data used to create scraping configuration snippet
type Scrape struct {
	MetricsPath string `json:"metricsPath,string,omitempty"`
	ScrapePort  int    `json:"scrapePort,string,omitempty"`
	ServiceName string `json:"serviceName"`
	ScrapeType  string `json:"scrapeType"`
}
