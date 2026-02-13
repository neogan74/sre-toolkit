package linter

import (
"bufio"
"context"
"fmt"
"os"
"path/filepath"

"strings"
)

// DockerfileLinter implements Linter for Dockerfiles
type DockerfileLinter struct{}

func NewDockerfileLinter() *DockerfileLinter {
return &DockerfileLinter{}
}

func (l *DockerfileLinter) Lint(ctx context.Context, path string) (*Result, error) {
result := &Result{Passed: true}

// Check filename convention
logName := filepath.Base(path)
if logName != "Dockerfile" && !strings.HasSuffix(logName, ".Dockerfile") && !strings.HasSuffix(logName, ".dockerfile") {
// Not a Dockerfile, skip (although caller should have filtered, but safety first)
return nil, nil
}

file, err := os.Open(path)
if err != nil {
return nil, fmt.Errorf("failed to open file: %w", err)
}
defer file.Close()

scanner := bufio.NewScanner(file)
lineNum := 0

// State tracking
hasUser := false
fromCount := 0

for scanner.Scan() {
lineNum++
line := strings.TrimSpace(scanner.Text())

// Skip comments and empty lines
if line == "" || strings.HasPrefix(line, "#") {
continue
}

// Check for specific instructions
parts := strings.Fields(line)
instruction := strings.ToUpper(parts[0])

switch instruction {
case "FROM":
fromCount++
if len(parts) > 1 {
imageObj := parts[1]
// Check for 'latest' tag or no tag
if !strings.Contains(imageObj, ":") || strings.HasSuffix(imageObj, ":latest") {
result.Issues = append(result.Issues, Issue{
Severity: "Medium",
Message:  "Avoid using 'latest' tag or no tag in FROM instruction. Pin to a specific version or hash.",
File:     path,
Line:     lineNum,
})
}
// Check for platform flag necessity? (Maybe too strict)
}
// Reset user check on new stage
hasUser = false

case "MAINTAINER":
result.Issues = append(result.Issues, Issue{
Severity: "Low",
Message:  "MAINTAINER instruction is deprecated. Use LABEL maintainer=\"name\" instead.",
File:     path,
Line:     lineNum,
})

case "RUN":
// Check for sudo
if strings.Contains(line, "sudo ") {
result.Issues = append(result.Issues, Issue{
Severity: "High",
Message:  "Avoid using 'sudo' in RUN instructions. Use USER root if necessary, but revert later.",
File:     path,
Line:     lineNum,
})
}
// Check for apt-get upgrade
if strings.Contains(line, "apt-get upgrade") || strings.Contains(line, "apt upgrade") {
result.Issues = append(result.Issues, Issue{
Severity: "Medium",
Message:  "Avoid 'apt-get upgrade' or 'apt upgrade'. It can cause non-deterministic builds.",
File:     path,
Line:     lineNum,
})
}
// Check for unpinned package installs (hard to do robustly with regex, skipping for now)

case "ADD":
// Suggest COPY
isTar := strings.HasSuffix(line, ".tar") || strings.HasSuffix(line, ".tar.gz") || strings.HasSuffix(line, ".tgz")
isUrl := strings.Contains(line, "http://") || strings.Contains(line, "https://")
if !isTar && !isUrl {
result.Issues = append(result.Issues, Issue{
Severity: "Low",
Message:  "Prefer COPY over ADD unless extracting archives or downloading URLs.",
File:     path,
Line:     lineNum,
})
}

case "USER":
hasUser = true

case "WORKDIR":
if len(parts) > 1 {
dir := parts[1]
if !filepath.IsAbs(dir) && !strings.HasPrefix(dir, "$") { // allow env vars
result.Issues = append(result.Issues, Issue{
Severity: "Low",
Message:  "WORKDIR paths should be absolute.",
File:     path,
Line:     lineNum,
})
}
}
}
}

if err := scanner.Err(); err != nil {
return nil, fmt.Errorf("error reading file: %w", err)
}

// End of file checks
if fromCount > 0 && !hasUser {
result.Issues = append(result.Issues, Issue{
Severity: "Medium",
Message:  "Dockerfile does not switch to a non-root USER. (Last stage)",
File:     path,
Line:     lineNum, // Point to end of file
})
}

if len(result.Issues) > 0 {
result.Passed = false
}

return result, nil
}
