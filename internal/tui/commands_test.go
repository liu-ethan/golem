package tui

import (
	"strings"
	"testing"

	"github.com/tencent-docs/golem/internal/approval"
)

func TestParseSlashCommand(t *testing.T) {
	cmd, args := parseSlashCommand("/permissions ask-before-edit")
	if cmd != "permissions" || len(args) != 1 || args[0] != "ask-before-edit" {
		t.Fatalf("got cmd=%q args=%v", cmd, args)
	}
}

func TestDispatchSlashHelp(t *testing.T) {
	r := dispatchSlash("/help")
	if !r.handled || r.message == "" {
		t.Fatalf("help result = %+v", r)
	}
	if !strings.Contains(r.message, "Shift+Tab") {
		t.Error("help should mention Shift+Tab")
	}
}

func TestDispatchSlashPermissionsMode(t *testing.T) {
	r := dispatchSlash("/permissions plan")
	if !r.handled || r.setMode != approval.ModePlan {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashPermissionsPage(t *testing.T) {
	r := dispatchSlash("/permissions")
	if !r.handled || r.openPage != PagePermissions {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashSessionsPage(t *testing.T) {
	r := dispatchSlash("/sessions")
	if !r.handled || r.openPage != PageSessions {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashExit(t *testing.T) {
	r := dispatchSlash("/exit")
	if !r.handled || !r.quit {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashUnknown(t *testing.T) {
	r := dispatchSlash("/unknown-cmd")
	if !r.handled || r.message == "" {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashPlainTextNotHandled(t *testing.T) {
	for _, input := range []string{"你好", "hello", "read main.go"} {
		r := dispatchSlash(input)
		if r.handled {
			t.Fatalf("input %q should not be handled as slash, got %+v", input, r)
		}
	}
}

func TestDispatchSlashBareSlash(t *testing.T) {
	r := dispatchSlash("/")
	if !r.handled || !strings.Contains(r.message, "未知命令") {
		t.Fatalf("result = %+v", r)
	}
}

func TestNormalizeApprovalMode(t *testing.T) {
	if got := normalizeApprovalMode("auto"); got != approval.ModeEditAutomatically {
		t.Fatalf("got %q", got)
	}
}

func TestDispatchSlashCompact(t *testing.T) {
	r := dispatchSlash("/compact keep file paths")
	if !r.handled || !r.compact || r.compactInstructions != "keep file paths" {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashUsage(t *testing.T) {
	r := dispatchSlash("/usage")
	if !r.handled || !r.showUsage {
		t.Fatalf("result = %+v", r)
	}
	r = dispatchSlash("/cost")
	if !r.handled || !r.showUsage {
		t.Fatalf("cost alias result = %+v", r)
	}
}

func TestDispatchSlashReview(t *testing.T) {
	r := dispatchSlash("/review working-tree")
	if !r.handled || !r.runReview || r.reviewTarget != "working-tree" {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashExport(t *testing.T) {
	r := dispatchSlash("/export out.md")
	if !r.handled || !r.doExport || r.exportPath != "out.md" {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashSkillsPage(t *testing.T) {
	r := dispatchSlash("/skills")
	if !r.handled || r.openPage != PageSkills {
		t.Fatalf("result = %+v", r)
	}
}

func TestDispatchSlashPlan(t *testing.T) {
	r := dispatchSlash("/plan refactor auth module")
	if !r.handled || r.runPlan != "refactor auth module" {
		t.Fatalf("result = %+v", r)
	}
}

func TestFormatTokens(t *testing.T) {
	if got := formatTokens(12400, 200000); got != "12.4k / 200k" {
		t.Fatalf("got %q", got)
	}
}

func TestApprovalValidateMode(t *testing.T) {
	if err := approvalValidateMode("plan"); err != nil {
		t.Fatal(err)
	}
	if err := approvalValidateMode("bogus"); err == nil {
		t.Fatal("expected error for bogus mode")
	}
}
