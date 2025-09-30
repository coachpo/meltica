package binance

func hasString(m map[string]any, k string) bool {
	v, ok := m[k]
	if !ok {
		return false
	}
	_, ok = v.(string)
	return ok
}
