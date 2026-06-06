// Package formats provides log line parsers for common log formats.
package formats

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Level represents log severity.
type Level string

// Log severity level constants.
const (
	LevelTrace   Level = "TRACE"
	LevelDebug   Level = "DEBUG"
	LevelInfo    Level = "INFO"
	LevelWarning Level = "WARN"
	LevelError   Level = "ERROR"
	LevelFatal   Level = "FATAL"
	LevelUnknown Level = "UNKNOWN"
)

// Entry is a parsed log line.
type Entry struct {
	Raw       string
	Timestamp time.Time
	Level     Level
	Message   string
	Fields    map[string]string
	Source    string // file/service name
	LineNum   int
}

// Parser parses a single log line into an Entry.
type Parser interface {
	Name() string
	Parse(line string) (*Entry, error)
	Detect(sample string) bool
}

// --- JSON (structured) parser ---

var jsonKeyRE = regexp.MustCompile(`"(\w+)"\s*:\s*"?([^",}\n]*)"?`)

// JSONParser handles structured JSON log lines (zerolog, zap, logrus JSON).
type JSONParser struct{}

// Name returns the name of the JSON parser.
func (p *JSONParser) Name() string { return "json" }

// Detect reports whether the sample looks like a JSON log line.
func (p *JSONParser) Detect(sample string) bool {
	s := strings.TrimSpace(sample)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

// Parse parses a JSON log line into an Entry.
func (p *JSONParser) Parse(line string) (*Entry, error) {
	entry := &Entry{Raw: line, Fields: make(map[string]string)}

	matches := jsonKeyRE.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		entry.Fields[m[1]] = m[2]
	}

	// Common field names across zerolog/zap/logrus
	entry.Message = firstOf(entry.Fields, "message", "msg", "m")
	entry.Level = parseLevel(firstOf(entry.Fields, "level", "lvl", "severity", "l"))
	entry.Timestamp = parseTime(firstOf(entry.Fields, "time", "ts", "timestamp", "t", "@timestamp"))

	if entry.Message == "" {
		return nil, fmt.Errorf("no message field found")
	}
	return entry, nil
}

// --- Logfmt parser ---

var logfmtPairRE = regexp.MustCompile(`(\w+)=("([^"]*)"|(\S*))`)

// LogfmtParser handles key=value log lines.
type LogfmtParser struct{}

// Name returns the name of the logfmt parser.
func (p *LogfmtParser) Name() string { return "logfmt" }

// Detect reports whether the sample looks like a logfmt log line.
func (p *LogfmtParser) Detect(sample string) bool {
	return strings.Contains(sample, "=") && !strings.HasPrefix(strings.TrimSpace(sample), "{")
}

// Parse parses a logfmt log line into an Entry.
func (p *LogfmtParser) Parse(line string) (*Entry, error) {
	entry := &Entry{Raw: line, Fields: make(map[string]string)}

	matches := logfmtPairRE.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		val := m[3]
		if val == "" {
			val = m[4]
		}
		entry.Fields[m[1]] = val
	}

	entry.Message = firstOf(entry.Fields, "msg", "message", "m")
	entry.Level = parseLevel(firstOf(entry.Fields, "level", "lvl", "severity"))
	entry.Timestamp = parseTime(firstOf(entry.Fields, "time", "ts", "t"))

	if entry.Message == "" {
		entry.Message = line
	}
	return entry, nil
}

// --- Combined/Common Apache/nginx access log parser ---

// combinedLogRE matches the combined log format:
// 127.0.0.1 - user [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "http://ref" "UA"
var combinedLogRE = regexp.MustCompile(
	`^(\S+)\s+\S+\s+(\S+)\s+\[([^\]]+)\]\s+"([^"]+)"\s+(\d+)\s+(\d+|-)\s*(?:"([^"]*)")?\s*(?:"([^"]*)")?`)

// AccessLogParser handles Apache/nginx combined/common access log format.
type AccessLogParser struct{}

// Name returns the name of the access log parser.
func (p *AccessLogParser) Name() string { return "access" }

// Detect reports whether the sample matches the Apache/nginx combined log format.
func (p *AccessLogParser) Detect(sample string) bool {
	return combinedLogRE.MatchString(sample)
}

// Parse parses an Apache/nginx access log line into an Entry.
func (p *AccessLogParser) Parse(line string) (*Entry, error) {
	m := combinedLogRE.FindStringSubmatch(line)
	if m == nil {
		return nil, fmt.Errorf("line does not match access log format")
	}

	fields := map[string]string{
		"remote_addr": m[1],
		"user":        m[2],
		"request":     m[4],
		"status":      m[5],
		"bytes":       m[6],
		"referer":     m[7],
		"user_agent":  m[8],
	}

	ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", m[3])
	if err != nil {
		ts = time.Time{}
	}
	status, err := strconv.Atoi(m[5])
	if err != nil {
		status = 0
	}

	level := LevelInfo
	if status >= 500 {
		level = LevelError
	} else if status >= 400 {
		level = LevelWarning
	}

	return &Entry{
		Raw:       line,
		Timestamp: ts,
		Level:     level,
		Message:   fmt.Sprintf("%s %s", m[4], m[5]),
		Fields:    fields,
	}, nil
}

// --- Syslog parser ---

// syslogRE matches: Oct 11 22:14:15 mymachine sshd[2137]: failed
var syslogRE = regexp.MustCompile(
	`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[(\d+)\])?:\s+(.*)$`)

// SyslogParser handles traditional syslog format.
type SyslogParser struct{}

// Name returns the name of the syslog parser.
func (p *SyslogParser) Name() string { return "syslog" }

// Detect reports whether the sample matches traditional syslog format.
func (p *SyslogParser) Detect(sample string) bool {
	return syslogRE.MatchString(sample)
}

// Parse parses a syslog line into an Entry.
func (p *SyslogParser) Parse(line string) (*Entry, error) {
	m := syslogRE.FindStringSubmatch(line)
	if m == nil {
		return nil, fmt.Errorf("line does not match syslog format")
	}

	ts, err := time.Parse("Jan  2 15:04:05", m[1])
	if err != nil || ts.IsZero() {
		ts, err = time.Parse("Jan _2 15:04:05", m[1])
		if err != nil {
			ts = time.Time{}
		}
	}

	fields := map[string]string{
		"host":    m[2],
		"program": m[3],
		"pid":     m[4],
	}

	return &Entry{
		Raw:       line,
		Timestamp: ts,
		Level:     detectLevelFromMessage(m[5]),
		Message:   m[5],
		Fields:    fields,
	}, nil
}

// --- Plaintext (fallback) parser ---

// PlainParser is a fallback that handles unstructured log lines.
// Tries to detect a timestamp and level prefix.
var (
	plainTimestampRE = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s+`)
	plainLevelRE     = regexp.MustCompile(`(?i)\b(TRACE|DEBUG|INFO|WARN(?:ING)?|ERROR|FATAL|CRITICAL)\b`)
)

// PlainParser is a fallback parser for unstructured log lines.
type PlainParser struct{}

// Name returns the name of the plain text parser.
func (p *PlainParser) Name() string { return "plain" }

// Detect always returns true as PlainParser is the fallback parser.
func (p *PlainParser) Detect(_ string) bool { return true } // always matches as fallback

// Parse parses an unstructured log line into an Entry.
func (p *PlainParser) Parse(line string) (*Entry, error) {
	entry := &Entry{Raw: line, Fields: make(map[string]string), Message: line}

	rest := line
	if m := plainTimestampRE.FindStringSubmatch(line); m != nil {
		for _, layout := range []string{
			time.RFC3339, time.RFC3339Nano,
			"2006-01-02 15:04:05",
			"2006-01-02 15:04:05.000",
		} {
			if ts, err := time.Parse(layout, m[1]); err == nil {
				entry.Timestamp = ts
				rest = line[len(m[0]):]
				break
			}
		}
	}

	if m := plainLevelRE.FindStringSubmatch(rest); m != nil {
		entry.Level = parseLevel(m[1])
		// Strip level prefix from message
		entry.Message = strings.TrimSpace(plainLevelRE.ReplaceAllString(rest, ""))
		if entry.Message == "" {
			entry.Message = rest
		}
	} else {
		entry.Level = detectLevelFromMessage(rest)
	}

	return entry, nil
}

// Detect auto-detects the best parser for a sample line.
func Detect(sample string) Parser {
	parsers := []Parser{
		&JSONParser{},
		&AccessLogParser{},
		&SyslogParser{},
		&LogfmtParser{},
	}
	for _, p := range parsers {
		if p.Detect(sample) {
			return p
		}
	}
	return &PlainParser{}
}

// All returns all registered parsers.
func All() []Parser {
	return []Parser{
		&JSONParser{},
		&AccessLogParser{},
		&SyslogParser{},
		&LogfmtParser{},
		&PlainParser{},
	}
}

// --- helpers ---

func firstOf(fields map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := fields[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

func parseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return LevelTrace
	case "DEBUG":
		return LevelDebug
	case "INFO", "INFORMATION":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarning
	case "ERROR", "ERR":
		return LevelError
	case "FATAL", "CRITICAL", "CRIT":
		return LevelFatal
	default:
		return LevelUnknown
	}
}

func detectLevelFromMessage(msg string) Level {
	m := plainLevelRE.FindStringSubmatch(msg)
	if m != nil {
		return parseLevel(m[1])
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "error") || strings.Contains(lower, "fail") {
		return LevelError
	}
	if strings.Contains(lower, "warn") {
		return LevelWarning
	}
	return LevelUnknown
}

var timeLayouts = []string{
	time.RFC3339Nano, time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.000",
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	// Unix timestamp (float or int)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return time.Unix(sec, nsec)
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
