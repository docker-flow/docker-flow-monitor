package server

import (
	"strings"
)

func GetScrapeFromEnv(env string, prefix []string) (key, value string) {
	for _, p := range prefix {
		if strings.HasPrefix(env, p+"_") {
			values := strings.Split(env, "=")
			key = values[0]
			value = values[1]
		}
	}
	return key, value
}

