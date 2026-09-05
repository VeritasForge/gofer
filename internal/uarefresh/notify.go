package uarefresh

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// MacNotify 는 macOS 알림 한 줄이다. Slack 전송이 실패했을 때의 대체 수단 (설계 6·7절).
func MacNotify(ctx context.Context, title, message string) error {
	script := fmt.Sprintf("display notification %s with title %s", appleScriptString(message), appleScriptString(title))
	return exec.CommandContext(ctx, "osascript", "-e", script).Run()
}

// appleScriptString 은 AppleScript 문자열 리터럴로 감싼다.
func appleScriptString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
