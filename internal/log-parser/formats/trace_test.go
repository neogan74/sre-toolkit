package formats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntryTraceID(t *testing.T) {
	json, err := (&JSONParser{}).Parse(`{"level":"error","msg":"boom","traceId":"abc123"}`)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", json.TraceID(nil))

	lf, err := (&LogfmtParser{}).Parse(`level=info msg=ok request-id=r-1 trace_id=t-9`)
	assert.NoError(t, err)
	assert.Equal(t, "t-9", lf.TraceID(nil), "trace_id outranks request_id")
	assert.Equal(t, "r-1", lf.TraceID([]string{"Request_ID"}), "custom key, case/separator-insensitive")
	assert.Equal(t, "", lf.TraceID([]string{"span"}))

	none, _ := (&PlainParser{}).Parse("no fields here")
	assert.Equal(t, "", none.TraceID(nil))

	empty, _ := (&LogfmtParser{}).Parse(`msg=x trace_id=`)
	assert.Equal(t, "", empty.TraceID(nil), "empty value is not an ID")
}
