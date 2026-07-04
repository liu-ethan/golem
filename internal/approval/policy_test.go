package approval

import "testing"

func TestPlanModeDeniesMutatingTools(t *testing.T) {
	p, err := New(ModePlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"write_file", "edit_file", "bash"} {
		if !p.IsDenied(tool, nil) {
			t.Errorf("plan: IsDenied(%s) = false, want true", tool)
		}
		if p.ShouldConfirm(tool, nil) {
			t.Errorf("plan: ShouldConfirm(%s) = true, want false", tool)
		}
	}
	for _, tool := range []string{"read_file", "list_dir", "grep"} {
		if p.IsDenied(tool, nil) {
			t.Errorf("plan: IsDenied(%s) = true, want false", tool)
		}
		if p.ShouldConfirm(tool, nil) {
			t.Errorf("plan: ShouldConfirm(%s) = true, want false", tool)
		}
	}
}

func TestAskBeforeEditConfirmsMutatingOnly(t *testing.T) {
	p, err := New(ModeAskBeforeEdit)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"write_file", "edit_file", "bash"} {
		if p.IsDenied(tool, nil) {
			t.Errorf("ask-before-edit: IsDenied(%s) = true, want false", tool)
		}
		if !p.ShouldConfirm(tool, nil) {
			t.Errorf("ask-before-edit: ShouldConfirm(%s) = false, want true", tool)
		}
	}
	for _, tool := range []string{"read_file", "list_dir", "grep"} {
		if p.ShouldConfirm(tool, nil) {
			t.Errorf("ask-before-edit: ShouldConfirm(%s) = true, want false", tool)
		}
	}
}

func TestAskModeConfirmsAllTools(t *testing.T) {
	p, err := New(ModeAsk)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"read_file", "write_file", "bash", "grep"} {
		if p.IsDenied(tool, nil) {
			t.Errorf("ask: IsDenied(%s) = true, want false", tool)
		}
		if !p.ShouldConfirm(tool, nil) {
			t.Errorf("ask: ShouldConfirm(%s) = false, want true", tool)
		}
	}
}

func TestEditAutomaticallyAllowsAll(t *testing.T) {
	p, err := New(ModeEditAutomatically)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"read_file", "write_file", "edit_file", "bash"} {
		if p.IsDenied(tool, nil) {
			t.Errorf("edit-automatically: IsDenied(%s) = true, want false", tool)
		}
		if p.ShouldConfirm(tool, nil) {
			t.Errorf("edit-automatically: ShouldConfirm(%s) = true, want false", tool)
		}
	}
}

func TestSetModeAndCycleMode(t *testing.T) {
	p, err := New(ModeAskBeforeEdit)
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode() != ModeAskBeforeEdit {
		t.Fatalf("Mode() = %q", p.Mode())
	}
	if err := p.SetMode(ModePlan); err != nil {
		t.Fatal(err)
	}
	if p.Mode() != ModePlan {
		t.Errorf("after SetMode(plan), Mode() = %q", p.Mode())
	}
	if err := p.SetMode("invalid"); err == nil {
		t.Error("SetMode(invalid) should error")
	}

	p, _ = New(ModeAskBeforeEdit)
	wantCycle := []string{ModeAsk, ModeEditAutomatically, ModePlan, ModeAskBeforeEdit}
	for _, want := range wantCycle {
		if got := p.CycleMode(); got != want {
			t.Errorf("CycleMode() = %q, want %q", got, want)
		}
	}
}

func TestNewInvalidMode(t *testing.T) {
	if _, err := New("bogus"); err == nil {
		t.Error("New(bogus) should error")
	}
}
