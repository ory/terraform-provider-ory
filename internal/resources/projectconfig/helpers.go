package projectconfig

// getNestedValue safely traverses nested maps to extract a value.
func getNestedValue(config map[string]interface{}, keys ...string) interface{} {
	current := interface{}(config)
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[key]
		if !ok {
			return nil
		}
	}
	return current
}

// getNestedString extracts a string from nested maps, returning ("", false) if not found.
func getNestedString(config map[string]interface{}, keys ...string) (string, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// getNestedBool extracts a bool from nested maps, returning (false, false) if not found.
func getNestedBool(config map[string]interface{}, keys ...string) (bool, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// getNestedFloat extracts a number from nested maps (JSON numbers are float64).
func getNestedFloat(config map[string]interface{}, keys ...string) (float64, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}
