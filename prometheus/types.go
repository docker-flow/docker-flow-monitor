package prometheus

type Alert struct {
	AlertName string `json:"alertName"`
	AlertNameFormatted string
	AlertFor string `json:"alertFor,omitempty"`
	AlertIf   string `json:"alertIf,omitempty"`
	ServiceName string `json:"serviceName"`
}

type Scrape struct {
	ScrapePort 	int `json:"scrapePort,string,omitempty"`
	ServiceName string `json:"serviceName"`
}

