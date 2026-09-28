package settings

import (
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"strconv"
	"strings"
)

// mergeCustomSettingSpecs keeps custom settings on the same seed, validation,
// and runtime paths as upstream settings. Core definitions take precedence.
func mergeCustomSettingSpecs(core []settingSpec) []settingSpec {
	result := append([]settingSpec(nil), core...)
	seen := make(map[string]bool, len(core))
	for _, spec := range core {
		seen[spec.fullKey()] = true
	}
	for _, item := range legacyCustomSettings() {
		key := item.Namespace + ":" + item.Key
		if seen[key] {
			continue
		}
		spec := settingSpec{Namespace: item.Namespace, Key: item.Key, ValueType: item.ValueType, Default: item.Value, Description: item.Description}
		spec.Validate = func(value string, _ string) error {
			switch item.ValueType {
			case "int":
				if _, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err != nil {
					return settingRule("integer", "")
				}
			case "bool":
				if err := boolValue()(value, key); err != nil {
					return err
				}
			}
			return validateLegacyCustomPatchItem(PatchItem{Namespace: item.Namespace, Key: item.Key, Value: value})
		}
		spec.Apply = func(cfg *config.Config, value string) {
			applyLegacyCustomItem(cfg, domainsettings.SystemSetting{Namespace: item.Namespace, Key: item.Key, Value: value})
		}
		// These modules read their settings directly from the repository.
		if item.Namespace == "agent_group" || item.Namespace == "platform_tools" {
			spec.Apply = nil
		}
		result = append(result, spec)
		seen[key] = true
	}
	return result
}
