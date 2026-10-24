package cmd

import (
	"strings"
	"testing"
)

func TestCompletionOffersBridgeNames(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"light", "on", ""}, "Desk Lamp"},
		{[]string{"room", "dim", ""}, "Office"},
		{[]string{"scene", "activate", ""}, "Office/Relax"},
		{[]string{"light", "color", "Desk Lamp", ""}, "warm_white"},
		{[]string{"api", ""}, "delete"},
	}
	for _, tt := range tests {
		out, err := execute(t, append([]string{"__complete"}, tt.args...)...)
		if err != nil {
			t.Fatalf("complete %v: %v", tt.args, err)
		}
		if !strings.Contains(out, tt.want+"\n") {
			t.Errorf("complete %v = %q, want %q", tt.args, out, tt.want)
		}
	}
}

func TestCompletionStopsAfterLastArgument(t *testing.T) {
	out, err := execute(t, "__complete", "light", "on", "Desk Lamp", "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if strings.Contains(out, "Desk Lamp") || !strings.HasPrefix(out, ":4\n") {
		t.Fatalf("complete = %q, want no candidates and no file completion", out)
	}
}

func TestSceneListGroupFilter(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "scene", "list", "--group", "office", "--names")
	if err != nil || out != "Office/Relax\n" {
		t.Fatalf("scene list --group office = %q, %v", out, err)
	}

	_, err = execute(t, "scene", "list", "--group", "Attic")
	if report, code := classifyError(err); code != exitNotFound || !strings.Contains(report.Error, "Known: Office") {
		t.Fatalf("err = %v (%s), want not found listing Office", err, report.Code)
	}
}
