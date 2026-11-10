package images

import (
	"net/url"
	"strings"
)

func PublicURL(baseURL, value string) string {
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)

	if err == nil && parsed.IsAbs() {
		return value
	}

	baseURL = strings.TrimRight(baseURL, "/")

	// Если в БД уже был относительный путь /images/...
	if strings.HasPrefix(value, "/images/") {
		if baseURL == "" {
			return value
		}

		return baseURL + value
	}

	imagePath := "/images/" + url.PathEscape(value)

	if baseURL == "" {
		return imagePath
	}

	return baseURL + imagePath
}
