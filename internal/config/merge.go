package config

// MergeDefaults merges default config into an existing config.
// It adds any missing categories (with enabled=false) and fills in missing encryption defaults.
// Existing user settings are never overwritten.
func MergeDefaults(existing *Config, defaults *Config) *Config {
	if existing.Categories == nil {
		existing.Categories = make(map[string]*CategoryConfig)
	}

	// Add missing categories from defaults (disabled so we don't surprise the user)
	for name, defCat := range defaults.Categories {
		if _, exists := existing.Categories[name]; !exists {
			// Copy the default but set enabled=false
			newCat := *defCat
			newCat.Enabled = false
			existing.Categories[name] = &newCat
		}
	}

	// Fill in missing encryption defaults
	if existing.Encryption.Extension == "" {
		existing.Encryption.Extension = defaults.Encryption.Extension
	}

	return existing
}
