package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runLint executes the root command with separate stdout/stderr buffers.
func runLint(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	if stdin != "" {
		root.SetIn(strings.NewReader(stdin))
	}
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestLintIntentCanonicalizesValid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "intent.md")
	os.WriteFile(p, []byte("# Demo\n## T · album\n- composer: Palestrina\n"), 0o644)
	out, errb, err := runLint(t, "", "lint-intent", p)
	if err != nil {
		t.Fatalf("unexpected err: %v (stderr=%s)", err, errb)
	}
	if !strings.Contains(out, "## T · album") || !strings.Contains(out, "- composer: Palestrina") {
		t.Errorf("stdout missing canonical content:\n%s", out)
	}
	if !strings.Contains(errb, "1 items") {
		t.Errorf("stderr missing summary line:\n%s", errb)
	}
}

func TestLintIntentReportsErrorsAndExitsNonzero(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.md")
	os.WriteFile(p, []byte("# Demo\n## T · album\n- pianist: X\n"), 0o644)
	_, errb, err := runLint(t, "", "lint-intent", p)
	if err == nil {
		t.Fatal("expected nonzero exit (error) for unknown role")
	}
	if !strings.Contains(errb, "unknown field or role") {
		t.Errorf("stderr missing diagnostic:\n%s", errb)
	}
}

func TestLintIntentWriteRewritesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "intent.md")
	// Credits out of order; --write should canonicalize the file.
	os.WriteFile(p, []byte("# Demo\n## T · album\n- soloist: A (oboe)\n- composer: B\n"), 0o644)
	_, _, err := runLint(t, "", "lint-intent", "--write", p)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got, _ := os.ReadFile(p)
	want := "# Demo\n\n## T · album\n- composer: B\n- soloist: A (oboe)\n"
	if string(got) != want {
		t.Errorf("file not canonicalized:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestLintIntentWriteStdinIsError(t *testing.T) {
	_, _, err := runLint(t, "# Demo\n## T · album\n- composer: B\n", "lint-intent", "--write", "-")
	if err == nil {
		t.Fatal("expected error: --write with stdin")
	}
}

func TestLintIntentReadsStdin(t *testing.T) {
	out, _, err := runLint(t, "# Demo\n## T · album\n- composer: B\n", "lint-intent", "-")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, "- composer: B") {
		t.Errorf("stdin not canonicalized to stdout:\n%s", out)
	}
}

// TestLintIntentWriteDoesNotClobberOnError pins a data-integrity invariant: when
// --write hits a validation error the original file must be left untouched (the
// error check returns before os.WriteFile). A future reorder that wrote the
// canonical form before checking HasError would overwrite the user's file with a
// half-baked result; this guards against that.
func TestLintIntentWriteDoesNotClobberOnError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.md")
	original := "# Demo\n## T · album\n- pianist: X\n"
	os.WriteFile(p, []byte(original), 0o644)
	_, _, err := runLint(t, "", "lint-intent", "--write", p)
	if err == nil {
		t.Fatal("expected nonzero exit for validation error")
	}
	got, _ := os.ReadFile(p)
	if string(got) != original {
		t.Errorf("--write clobbered the file on validation error:\n--- got ---\n%s\n--- want ---\n%s", got, original)
	}
}
