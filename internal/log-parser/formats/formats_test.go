package formats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- JSON parser ---

func TestJSONParser_Detect(t *testing.T) {
	p := &JSONParser{}
	assert.True(t, p.Detect(`{"level":"info","msg":"hello"}`))
	assert.False(t, p.Detect(`level=info msg=hello`))
	assert.False(t, p.Detect(`2024-01-01 INFO hello`))
}

func TestJSONParser_Parse_Zerolog(t *testing.T) {
	p := &JSONParser{}
	line := `{"level":"info","time":"2024-03-15T10:00:00Z","message":"server started","port":8080}`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelInfo, entry.Level)
	assert.Equal(t, "server started", entry.Message)
	assert.False(t, entry.Timestamp.IsZero())
}

func TestJSONParser_Parse_Logrus(t *testing.T) {
	p := &JSONParser{}
	line := `{"level":"error","msg":"database connection failed","time":"2024-03-15T10:00:00Z"}`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelError, entry.Level)
	assert.Equal(t, "database connection failed", entry.Message)
}

func TestJSONParser_Parse_NoMessage(t *testing.T) {
	p := &JSONParser{}
	line := `{"level":"info","time":"2024-03-15T10:00:00Z"}`
	_, err := p.Parse(line)
	assert.Error(t, err)
}

// --- Logfmt parser ---

func TestLogfmtParser_Detect(t *testing.T) {
	p := &LogfmtParser{}
	assert.True(t, p.Detect(`level=info msg="hello world" ts=2024-01-01`))
	assert.False(t, p.Detect(`{"level":"info"}`))
}

func TestLogfmtParser_Parse(t *testing.T) {
	p := &LogfmtParser{}
	line := `level=warn msg="disk space low" host=server1 ts=2024-03-15T10:00:00Z`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelWarning, entry.Level)
	assert.Equal(t, "disk space low", entry.Message)
	assert.Equal(t, "server1", entry.Fields["host"])
}

func TestLogfmtParser_Parse_QuotedMsg(t *testing.T) {
	p := &LogfmtParser{}
	line := `level=error msg="connection refused to 127.0.0.1:5432"`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelError, entry.Level)
}

// --- Access log parser ---

func TestAccessLogParser_Detect(t *testing.T) {
	p := &AccessLogParser{}
	line := `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326`
	assert.True(t, p.Detect(line))
	assert.False(t, p.Detect(`{"level":"info"}`))
}

func TestAccessLogParser_Parse_200(t *testing.T) {
	p := &AccessLogParser{}
	line := `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelInfo, entry.Level)
	assert.Contains(t, entry.Message, "200")
	assert.Equal(t, "127.0.0.1", entry.Fields["remote_addr"])
}

func TestAccessLogParser_Parse_500(t *testing.T) {
	p := &AccessLogParser{}
	line := `10.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "POST /api/data HTTP/1.1" 500 512`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelError, entry.Level)
}

func TestAccessLogParser_Parse_404(t *testing.T) {
	p := &AccessLogParser{}
	line := `10.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /notfound HTTP/1.1" 404 0`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelWarning, entry.Level)
}

func TestAccessLogParser_Parse_Invalid(t *testing.T) {
	p := &AccessLogParser{}
	_, err := p.Parse("this is not an access log line")
	assert.Error(t, err)
}

// --- Syslog parser ---

func TestSyslogParser_Detect(t *testing.T) {
	p := &SyslogParser{}
	line := `Oct 11 22:14:15 mymachine sshd[2137]: failed to authenticate user`
	assert.True(t, p.Detect(line))
}

func TestSyslogParser_Parse(t *testing.T) {
	p := &SyslogParser{}
	line := `Oct 11 22:14:15 mymachine sshd[2137]: failed to authenticate user`
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, "mymachine", entry.Fields["host"])
	assert.Equal(t, "sshd", entry.Fields["program"])
	assert.Equal(t, "2137", entry.Fields["pid"])
	assert.Equal(t, "failed to authenticate user", entry.Message)
}

func TestSyslogParser_Parse_Invalid(t *testing.T) {
	p := &SyslogParser{}
	_, err := p.Parse("not a syslog line")
	assert.Error(t, err)
}

// --- Plain parser ---

func TestPlainParser_Parse_WithTimestampAndLevel(t *testing.T) {
	p := &PlainParser{}
	line := "2024-03-15 10:00:00 ERROR database connection failed"
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelError, entry.Level)
	assert.False(t, entry.Timestamp.IsZero())
	assert.Equal(t, time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC), entry.Timestamp)
}

func TestPlainParser_Parse_NoTimestamp(t *testing.T) {
	p := &PlainParser{}
	line := "WARN: disk usage above 90%"
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, LevelWarning, entry.Level)
}

func TestPlainParser_Parse_Fallback(t *testing.T) {
	p := &PlainParser{}
	line := "some random log line without structure"
	entry, err := p.Parse(line)
	require.NoError(t, err)
	assert.Equal(t, line, entry.Raw)
}

// --- Detect ---

func TestDetect_JSON(t *testing.T) {
	p := Detect(`{"level":"info","msg":"hello"}`)
	assert.Equal(t, "json", p.Name())
}

func TestDetect_Access(t *testing.T) {
	line := `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET / HTTP/1.0" 200 100`
	p := Detect(line)
	assert.Equal(t, "access", p.Name())
}

func TestDetect_Logfmt(t *testing.T) {
	p := Detect(`level=info msg="hello" host=server1`)
	assert.Equal(t, "logfmt", p.Name())
}

func TestDetect_Plain_Fallback(t *testing.T) {
	p := Detect("some plain log line")
	assert.Equal(t, "plain", p.Name())
}

// --- parseLevel ---

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"WARN", LevelWarning},
		{"WARNING", LevelWarning},
		{"error", LevelError},
		{"ERR", LevelError},
		{"debug", LevelDebug},
		{"FATAL", LevelFatal},
		{"CRITICAL", LevelFatal},
		{"trace", LevelTrace},
		{"unknown_level", LevelUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseLevel(tt.input))
		})
	}
}

// --- parseTime ---

func TestParseTime_RFC3339(t *testing.T) {
	ts := parseTime("2024-03-15T10:00:00Z")
	assert.Equal(t, 2024, ts.Year())
	assert.Equal(t, time.March, ts.Month())
}

func TestParseTime_UnixTimestamp(t *testing.T) {
	ts := parseTime("1710500000")
	assert.False(t, ts.IsZero())
}

func TestParseTime_Invalid(t *testing.T) {
	ts := parseTime("not-a-time")
	assert.True(t, ts.IsZero())
}
