package util

type Envelope map[string]any

func Error(message string) Envelope {
	return Envelope{"error": message}
}

func Data(key string, value any) Envelope {
	return Envelope{key: value}
}
