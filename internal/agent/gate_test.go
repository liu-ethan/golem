package agent

import (
	"testing"

	"github.com/tencent-docs/golem/internal/approval"
	"github.com/tencent-docs/golem/internal/rules"
)

func TestRulesGateDenyBash(t *testing.T) {
	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	gate := GateWithRules([]rules.Rule{
		{Action: "deny", Pattern: "rm -rf *"},
	}, GateFromPolicy(policy))

	input := map[string]any{"command": "rm -rf /tmp/x"}
	if !gate.IsDenied("bash", input) {
		t.Error("expected bash deny by rule")
	}
	if gate.ShouldConfirm("bash", input) {
		t.Error("deny should not require confirm")
	}
	rd, ok := gate.(RuleDenier)
	if !ok || !rd.DeniedByRule("bash", input) {
		t.Error("expected DeniedByRule true")
	}
}

func TestRulesGateAskOverridesEditAutomatically(t *testing.T) {
	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	gate := GateWithRules([]rules.Rule{
		{Action: "ask", Pattern: "curl *"},
	}, GateFromPolicy(policy))

	input := map[string]any{"command": "curl https://example.com"}
	if gate.IsDenied("bash", input) {
		t.Error("ask rule should not deny")
	}
	if !gate.ShouldConfirm("bash", input) {
		t.Error("ask rule should require confirm under edit-automatically")
	}
}

func TestRulesGateDoesNotAffectFileTools(t *testing.T) {
	policy, err := approval.New(approval.ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	gate := GateWithRules([]rules.Rule{
		{Action: "deny", Pattern: "*"},
	}, GateFromPolicy(policy))

	input := map[string]any{"path": "foo.txt", "content": "x"}
	if gate.IsDenied("write_file", input) {
		t.Error("rules should not apply to write_file")
	}
}

func TestRulesGatePlanStillDeniesBash(t *testing.T) {
	policy, err := approval.New(approval.ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	gate := GateWithRules([]rules.Rule{
		{Action: "allow", Pattern: "go *"},
	}, GateFromPolicy(policy))

	input := map[string]any{"command": "go test ./..."}
	if !gate.IsDenied("bash", input) {
		t.Error("plan mode should deny bash even when rule allows")
	}
}
