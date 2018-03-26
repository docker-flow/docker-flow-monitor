package prometheus

import "encoding/json"

// ScrapeConfig configures a scraping unit for Prometheus.
type ScrapeConfig struct {
	// The job name to which the job label is set by default.
	JobName string `yaml:"job_name"`
	// Indicator whether the scraped metrics should remain unmodified.
	HonorLabels bool `yaml:"honor_labels,omitempty"`
	// A set of query parameters with which the target is scraped.
	Params map[string][]string `yaml:"params,omitempty"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval string `yaml:"scrape_interval,omitempty"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout string `yaml:"scrape_timeout,omitempty"`
	// The HTTP resource path on which to fetch metrics from targets.
	MetricsPath string `yaml:"metrics_path,omitempty"`
	// The URL scheme with which to fetch metrics from targets.
	Scheme string `yaml:"scheme,omitempty"`
	// More than this many samples post metric-relabelling will cause the scrape to fail.
	SampleLimit uint `yaml:"sample_limit,omitempty"`

	ServiceDiscoveryConfig ServiceDiscoveryConfig `yaml:",inline"`
	HTTPClientConfig       HTTPClientConfig       `yaml:",inline"`

	// List of target relabel configurations.
	RelabelConfigs []*RelabelConfig `yaml:"relabel_configs,omitempty"`
	// List of metric relabel configurations.
	MetricRelabelConfigs []*RelabelConfig `yaml:"metric_relabel_configs,omitempty"`
}

// RemoteReadConfig is the configuration for reading from remote storage.
type RemoteReadConfig struct {
	URL           string `yaml:"url"`
	RemoteTimeout string `yaml:"remote_timeout,omitempty"`
	ReadRecent    bool   `yaml:"read_recent,omitempty"`

	HTTPClientConfig HTTPClientConfig `yaml:",inline"`

	// RequiredMatchers is an optional list of equality matchers which have to
	// be present in a selector to query the remote read endpoint.
	RequiredMatchers map[string]string `yaml:"required_matchers,omitempty"`
}

// QueueConfig is the configuration for the queue used to write to remote
// storage.
type QueueConfig struct {
	// Number of samples to buffer per shard before we start dropping them.
	Capacity int `yaml:"capacity,omitempty"`

	// Max number of shards, i.e. amount of concurrency.
	MaxShards int `yaml:"max_shards,omitempty"`

	// Maximum number of samples per send.
	MaxSamplesPerSend int `yaml:"max_samples_per_send,omitempty"`

	// Maximum time sample will wait in buffer.
	BatchSendDeadline string `yaml:"batch_send_deadline,omitempty"`

	// Max number of times to retry a batch on recoverable errors.
	MaxRetries int `yaml:"max_retries,omitempty"`

	// On recoverable errors, backoff exponentially.
	MinBackoff string `yaml:"min_backoff,omitempty"`
	MaxBackoff string `yaml:"max_backoff,omitempty"`
}

// RemoteWriteConfig is the configuration for writing to remote storage.
type RemoteWriteConfig struct {
	URL                 string           `yaml:"url"`
	RemoteTimeout       string           `yaml:"remote_timeout,omitempty"`
	WriteRelabelConfigs []*RelabelConfig `yaml:"write_relabel_configs,omitempty"`

	HTTPClientConfig HTTPClientConfig `yaml:",inline"`
	QueueConfig      QueueConfig      `yaml:"queue_config,omitempty"`
}

// TargetGroup is a set of targets with a common label set(production , test, staging etc.).
type TargetGroup struct {
	// Targets is a list of targets identified by a label set. Each target is
	// uniquely identifiable in the group by its address label.
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty"`
	// Labels is a set of labels that is common across all targets in the group.
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Source is an identifier that describes a group of targets.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
}

// DNSSDConfig is the configuration for DNS based service discovery.
type DNSSDConfig struct {
	Names           []string `yaml:"names"`
	RefreshInterval string   `yaml:"refresh_interval,omitempty"`
	Type            string   `yaml:"type"`
	Port            int      `yaml:"port"` // Ignored for SRV records
}

// SDConfig is the configuration for file based discovery.
type SDConfig struct {
	Files           []string `yaml:"files"`
	RefreshInterval string   `yaml:"refresh_interval,omitempty"`
}

// FileStaticConfig configures File-based service discovery
type FileStaticConfig []*TargetGroup

// ServiceDiscoveryConfig configures lists of different service discovery mechanisms.
type ServiceDiscoveryConfig struct {
	// List of labeled target groups for this job.
	StaticConfigs []*TargetGroup `yaml:"static_configs,omitempty"`
	// List of DNS service discovery configurations.
	DNSSDConfigs []*DNSSDConfig `yaml:"dns_sd_configs,omitempty"`
	// List of file service discovery configurations.
	FileSDConfigs []*SDConfig `yaml:"file_sd_configs,omitempty"`
}

// BasicAuth contains basic HTTP authentication credentials.
type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// TLSConfig configures the options for TLS connections.
type TLSConfig struct {
	// The CA cert to use for the targets.
	CAFile string `yaml:"ca_file,omitempty"`
	// The client cert file for the targets.
	CertFile string `yaml:"cert_file,omitempty"`
	// The client key file for the targets.
	KeyFile string `yaml:"key_file,omitempty"`
	// Used to verify the hostname for the targets.
	ServerName string `yaml:"server_name,omitempty"`
	// Disable target certificate validation.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// HTTPClientConfig configures an HTTP client.
type HTTPClientConfig struct {
	// The HTTP basic authentication credentials for the targets.
	BasicAuth *BasicAuth `yaml:"basic_auth,omitempty"`
	// The bearer token for the targets.
	BearerToken string `yaml:"bearer_token,omitempty"`
	// The bearer token file for the targets.
	BearerTokenFile string `yaml:"bearer_token_file,omitempty"`
	// HTTP proxy server to use to connect to the targets.
	ProxyURL string `yaml:"proxy_url,omitempty"`
	// TLSConfig to use to connect to the targets.
	TLSConfig TLSConfig `yaml:"tls_config,omitempty"`
}

// AlertmanagerConfig configures how Alertmanagers can be discovered and communicated with.
type AlertmanagerConfig struct {
	ServiceDiscoveryConfig ServiceDiscoveryConfig `yaml:",inline"`
	HTTPClientConfig       HTTPClientConfig       `yaml:",inline"`

	// The URL scheme to use when talking to Alertmanagers.
	Scheme string `yaml:"scheme,omitempty"`
	// Path prefix to add in front of the push endpoint path.
	PathPrefix string `yaml:"path_prefix,omitempty"`
	// The timeout used when sending alerts.
	Timeout string `yaml:"timeout,omitempty"`

	// List of Alertmanager relabel configurations.
	RelabelConfigs []*RelabelConfig `yaml:"relabel_configs,omitempty"`
}

// RelabelConfig is the configuration for relabeling of target label sets.
type RelabelConfig struct {
	// A list of labels from which values are taken and concatenated
	// with the configured separator in order.
	SourceLabels []string `yaml:"source_labels,flow,omitempty"`
	// Separator is the string between concatenated values from the source labels.
	Separator string `yaml:"separator,omitempty"`
	// Regex against which the concatenation is matched.
	Regex string `yaml:"regex,omitempty"`
	// Modulus to take of the hash of concatenated values from the source labels.
	Modulus uint64 `yaml:"modulus,omitempty"`
	// TargetLabel is the label to which the resulting string is written in a replacement.
	// Regexp interpolation is allowed for the replace action.
	TargetLabel string `yaml:"target_label,omitempty"`
	// Replacement is the regex replacement pattern to be used.
	Replacement string `yaml:"replacement,omitempty"`
	// Action is the action to be performed for the relabeling.
	Action string `yaml:"action,omitempty"`
}

// AlertingConfig configures alerting and alertmanager related configs.
type AlertingConfig struct {
	AlertRelabelConfigs []*RelabelConfig      `yaml:"alert_relabel_configs,omitempty"`
	AlertmanagerConfigs []*AlertmanagerConfig `yaml:"alertmanagers,omitempty"`
}

// GlobalConfig configures values that are used across other configuration
// objects.
type GlobalConfig struct {
	// How frequently to scrape targets by default.
	ScrapeInterval string `yaml:"scrape_interval,omitempty"`
	// The default timeout when scraping targets.
	ScrapeTimeout string `yaml:"scrape_timeout,omitempty"`
	// How frequently to evaluate rules by default.
	EvaluationInterval string `yaml:"evaluation_interval,omitempty"`
	// The labels to add to any timeseries that this Prometheus instance scrapes.
	ExternalLabels map[string]string `yaml:"external_labels,omitempty"`
}

// Config is the top-level configuration for Prometheus's config files.
type Config struct {
	GlobalConfig   GlobalConfig    `yaml:"global,omitempty"`
	AlertingConfig AlertingConfig  `yaml:"alerting,omitempty"`
	RuleFiles      []string        `yaml:"rule_files,omitempty"`
	ScrapeConfigs  []*ScrapeConfig `yaml:"scrape_configs,omitempty"`

	RemoteWriteConfigs []*RemoteWriteConfig `yaml:"remote_write,omitempty"`
	RemoteReadConfigs  []*RemoteReadConfig  `yaml:"remote_read,omitempty"`
}

// Alert defines data used to create alert configuration snippet
type Alert struct {
	AlertAnnotations   map[string]string `json:"alertAnnotations,omitempty"`
	AlertFor           string            `json:"alertFor,omitempty"`
	AlertIf            string            `json:"alertIf,omitempty"`
	AlertLabels        map[string]string `json:"alertLabels,omitempty"`
	AlertName          string            `json:"alertName"`
	AlertPersistent    bool              `json:"alertPersistent"`
	AlertNameFormatted string
	ServiceName        string `json:"serviceName"`
	Replicas           int    `json:"replicas"`
}

// NodeIP defines a node/addr pair
type NodeIP struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
	ID   string `json:"id"`
}

// NodeIPSet is a set of NodeIPs
type NodeIPSet map[NodeIP]struct{}

// Add node to set
func (ns NodeIPSet) Add(name, addr, id string) {
	ns[NodeIP{Name: name, Addr: addr, ID: id}] = struct{}{}
}

// Equal returns true when NodeIPSets contain the same elements
func (ns NodeIPSet) Equal(other NodeIPSet) bool {

	if ns.Cardinality() != other.Cardinality() {
		return false
	}

	for ip := range ns {
		if _, ok := other[ip]; !ok {
			return false
		}
	}
	return true
}

// Cardinality returns the size of set
func (ns NodeIPSet) Cardinality() int {
	return len(ns)
}

// MarshalJSON creates JSON array from NodeIPSet
func (ns NodeIPSet) MarshalJSON() ([]byte, error) {
	items := make([][]string, 0, ns.Cardinality())

	for elem := range ns {
		items = append(items, []string{elem.Name, elem.Addr, elem.ID})
	}
	return json.Marshal(items)
}

// UnmarshalJSON recreates NodeIPSet from a JSON array
func (ns *NodeIPSet) UnmarshalJSON(b []byte) error {

	items := [][]string{}
	err := json.Unmarshal(b, &items)
	if err != nil {
		return err
	}

	for _, item := range items {
		nodeIP := NodeIP{Name: item[0], Addr: item[1]}
		if len(item) == 3 {
			nodeIP.ID = item[2]
		}
		(*ns)[nodeIP] = struct{}{}
	}

	return nil
}

// Scrape defines data used to create scraping configuration snippet
type Scrape struct {
	MetricsPath    string             `json:"metricsPath,string,omitempty"`
	ScrapeInterval string             `json:"scrapeInterval,string,omitempty"`
	ScrapeLabels   *map[string]string `json:"scrapeLabels,omitempty"`
	ScrapePort     int                `json:"scrapePort,string,omitempty"`
	ScrapeTimeout  string             `json:"scrapeTimeout,string,omitempty"`
	ScrapeType     string             `json:"scrapeType"`
	ServiceName    string             `json:"serviceName"`
	NodeInfo       NodeIPSet          `json:"nodeInfo,omitempty"`
}
