package routing

import (
	"bytes"
	"strconv"

	gojson "github.com/goccy/go-json"
)

func decodePayloadFields(raw []byte) (map[string]gojson.RawMessage, error) {
	dec := gojson.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var fields map[string]gojson.RawMessage
	if err := dec.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func eventTypeFromPayload(raw []byte) (string, error) {
	fields, err := decodePayloadFields(raw)
	if err != nil {
		return "", err
	}
	if rawEvent, ok := fields["e"]; ok {
		var event string
		if err := gojson.Unmarshal(rawEvent, &event); err != nil {
			return "", err
		}
		return event, nil
	}
	return "", nil
}

func extractInt64Field(raw []byte, key string) (int64, bool, error) {
	fields, err := decodePayloadFields(raw)
	if err != nil {
		return 0, false, err
	}
	value, ok := fields[key]
	if !ok {
		return 0, false, nil
	}
	var num gojson.Number
	if err := gojson.Unmarshal(value, &num); err != nil {
		return 0, false, err
	}
	millis, err := strconv.ParseInt(num.String(), 10, 64)
	if err != nil {
		return 0, false, err
	}
	return millis, true, nil
}
