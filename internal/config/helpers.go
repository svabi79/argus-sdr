package config

func BoolValue(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func boolPtr(v bool) *bool {
	return &v
}
