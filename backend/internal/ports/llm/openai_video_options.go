package llm

import (
	"regexp"
	"strings"
)

// SanitizeOpenAIVideoOptions normalizes the shared billing and provider request options.
func SanitizeOpenAIVideoOptions(options map[string]any) {
	if len(options) == 0 {
		return
	}
	duration, ok := IntegerOption(options, "duration")
	if !ok || duration < 1 || duration > 15 {
		duration, ok = IntegerOption(options, "seconds")
	}
	if ok && duration >= 1 && duration <= 15 {
		options["duration"] = duration
	} else {
		delete(options, "duration")
	}
	resolution := strings.ToLower(stringOption(options, "resolution"))
	if !openAIVideoResolutionPattern.MatchString(resolution) {
		switch strings.ToLower(stringOption(options, "size")) {
		case "854x480", "480p":
			resolution = "480p"
		case "1280x720", "720p":
			resolution = "720p"
		case "1920x1080", "1080p":
			resolution = "1080p"
		default:
			resolution = ""
		}
	}
	if resolution != "" {
		options["resolution"] = resolution
	} else {
		delete(options, "resolution")
	}
	aspect := strings.ToLower(stringOption(options, "aspect_ratio"))
	switch aspect {
	case "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3":
		options["aspect_ratio"] = aspect
	default:
		delete(options, "aspect_ratio")
	}
	for key := range options {
		switch key {
		case "duration", "resolution", "aspect_ratio":
		default:
			delete(options, key)
		}
	}
}

var openAIVideoResolutionPattern = regexp.MustCompile(`^\d{2,5}p$|^\d{1,5}k$|^\d{2,5}x\d{2,5}$`)
