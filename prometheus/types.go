package prometheus

type Alert struct {
	AlertAnnotations   map[string]string `json:"alertAnnotations,omitempty"`
	AlertFor           string `json:"alertFor,omitempty"`
	AlertIf            string `json:"alertIf,omitempty"`
	AlertLabels        map[string]string `json:"alertLabels,omitempty"`
	AlertName          string `json:"alertName"`
	AlertNameFormatted string
	ServiceName        string `json:"serviceName"`
}

type Scrape struct {
	ScrapePort 	int `json:"scrapePort,string,omitempty"`
	ServiceName string `json:"serviceName"`
	ScrapeType string
}

