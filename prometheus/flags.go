package prometheus

import (
	"fmt"
	"os"
	"strings"
)

// EnvToPrometheusFlags converts environmental variables into a string of
// flags to be used with prometheus v2 CLI
func EnvToPrometheusFlags(prefix string) []string {

	flags := []string{}
	for _, e := range os.Environ() {
		if key, value := getArgFromEnv(e, "ARG"); len(key) > 0 {
			key = strings.Replace(key, "_", ".", -1)

			if key == "web.enable-remote-shutdown" {
				if value == "true" {
					key = "web.enable-lifecycle"
					value = ""
				} else {
					continue
				}
			} else if key == "storage.local.path" {
				key = "storage.tsdb.path"
			} else if key == "storage.local.retention" {
				key = "storage.tsdb.retention"
			} else if key == "query.staleness-delta" {
				key = "query.lookback-delta"
			} else if key == "alertmanager.url" {
				continue
			}

			var flag string
			if len(value) > 0 {
				flag = fmt.Sprintf("--%s=\"%s\"", key, value)
			} else {
				flag = fmt.Sprintf("--%s", key)
			}

			flags = append(flags, flag)
		}
	}

	return flags
}
