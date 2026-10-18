package formats

import "strings"

// DefaultTraceKeys are the field names checked, in priority order, when
// looking for an ID that ties log lines of one request together. Matching
// ignores case and the separators "_", "-" and ".", so "trace_id", "traceId"
// and "Trace-ID" are all the same key.
var DefaultTraceKeys = []string{"trace_id", "request_id", "correlation_id", "req_id", "x_request_id"}

func normKey(k string) string {
	return strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(k))
}

// TraceID returns the correlation ID of the entry, or "" if none of keys
// (DefaultTraceKeys when empty) is present with a non-empty value.
func (e *Entry) TraceID(keys []string) string {
	if len(e.Fields) == 0 {
		return ""
	}
	if len(keys) == 0 {
		keys = DefaultTraceKeys
	}
	norm := make(map[string]string, len(e.Fields))
	for k, v := range e.Fields {
		if v != "" {
			norm[normKey(k)] = v
		}
	}
	for _, k := range keys {
		if v, ok := norm[normKey(k)]; ok {
			return v
		}
	}
	return ""
}
