package api

import (
	"strings"
)

func IsBot(userAgent string) bool {
    // basic check for common crawlers in the User-Agent string
    userAgent = strings.ToLower(userAgent)
    return strings.Contains(userAgent, "googlebot") ||
           strings.Contains(userAgent, "bingbot") ||
           strings.Contains(userAgent, "slurp") ||
           strings.Contains(userAgent, "duckduckbot") ||
           strings.Contains(userAgent, "baiduspider") ||
           strings.Contains(userAgent, "yandexbot") ||
           strings.Contains(userAgent, "facebot") ||
           strings.Contains(userAgent, "ia_archiver")
}
