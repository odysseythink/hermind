package scraper

import (
	"net/url"
	"strings"
)

func isYouTubeURL(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	return strings.Contains(u.Host, "youtube.com") || strings.Contains(u.Host, "youtu.be")
}

func validateURL(link string) string {
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		link = "https://" + link
	}
	return link
}

func validURL(link string) bool {
	u, err := url.Parse(link)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
