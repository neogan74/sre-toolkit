package linter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerfileLinter_Lint(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		content       string
		expectedIssues int
		issuePatterns []string
	}{
		{
			name: "Good Dockerfile",
			content: `
FROM alpine:3.14
LABEL maintainer="me@example.com"
WORKDIR /app
COPY app /app/
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
CMD ["./app"]
`,
			expectedIssues: 0,
		},
		{
			name: "Bad Dockerfile - Latest Tag",
			content: `
FROM alpine:latest
CMD ["echo", "hello"]
`,
			expectedIssues: 2, // Latest tag + No User
			issuePatterns:  []string{"'latest' tag", "non-root USER"},
		},
		{
			name: "Bad Dockerfile - No Tag",
			content: `
FROM ubuntu
CMD ["echo", "hello"]
`,
			expectedIssues: 2, // No tag + No User
			issuePatterns:  []string{"'latest' tag", "non-root USER"},
		},
		{
			name: "Bad Dockerfile - MAINTAINER",
			content: `
FROM alpine:3.14
MAINTAINER me@example.com
USER appuser
`,
			expectedIssues: 1,
			issuePatterns:  []string{"MAINTAINER instruction is deprecated"},
		},
		{
			name: "Bad Dockerfile - Sudo and Upgrade",
			content: `
FROM ubuntu:20.04
RUN sudo apt-get update
RUN apt-get upgrade -y
USER appuser
`,
			expectedIssues: 2,
			issuePatterns:  []string{"sudo", "apt-get upgrade"},
		},
		{
			name: "Bad Dockerfile - ADD instead of COPY",
			content: `
FROM alpine:3.14
ADD myfile.txt /app/
USER appuser
`,
			expectedIssues: 1,
			issuePatterns:  []string{"Prefer COPY over ADD"},
		},
		{
			name: "Good Dockerfile - ADD with URL",
			content: `
FROM alpine:3.14
ADD https://example.com/file.tar.gz /tmp/
USER appuser
`,
			expectedIssues: 0,
		},
		{
			name: "Bad Dockerfile - Relative WORKDIR",
			content: `
FROM alpine:3.14
WORKDIR app
USER appuser
`,
			expectedIssues: 1,
			issuePatterns:  []string{"WORKDIR paths should be absolute"},
		},
		{
			name: "Multi-stage Dockerfile - User in last stage only",
			content: `
FROM golang:1.20 AS builder
WORKDIR /src
COPY . .
RUN go build -o app .

FROM alpine:3.14
WORKDIR /app
COPY --from=builder /src/app .
USER appuser
CMD ["./app"]
`,
			expectedIssues: 0,
		},
	}

	linter := NewDockerfileLinter()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, "Dockerfile")
			err := os.WriteFile(path, []byte(tt.content), 0644)
			require.NoError(t, err)

			result, err := linter.Lint(ctx, path)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tt.expectedIssues, len(result.Issues), "Issue count mismatch")

			for _, pattern := range tt.issuePatterns {
				found := false
				for _, issue := range result.Issues {
					if strings.Contains(issue.Message, pattern) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected to find issue containing '%s', but found issues: %v", pattern, result.Issues)
			}
		})
	}
}
