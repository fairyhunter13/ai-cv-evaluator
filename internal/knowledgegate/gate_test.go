// Package knowledgegate holds no code. It exists so `go test ./...` asserts that the
// knowledge/ bundle is actually gated on this checkout -- the bundle passing okf says
// nothing about whether anything runs okf.
package knowledgegate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const okfPin = "github.com/fairyhunter13/okf/cmd/okf@v0.1.0"

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("not in a git checkout: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func read(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return string(body)
}

func TestTheMakeTargetBlocksRatherThanReports(t *testing.T) {
	mk := read(t, "Makefile")
	i := strings.Index(mk, "\nlint-knowledge:")
	if i < 0 {
		t.Fatal("Makefile has no lint-knowledge target")
	}
	target := mk[i:]
	if j := strings.Index(target[1:], "\n\n"); j >= 0 {
		target = target[:j+1]
	}
	// -Werror, because a broken link is a *warning*: plain `check` prints it and exits 0.
	if !strings.Contains(target, "-Werror") {
		t.Error("lint-knowledge does not pass -Werror, so a dangling link exits 0")
	}
	// A target that always exits 0 reports nothing a build can act on.
	if strings.Contains(target, "|| true") || strings.Contains(target, "- ") {
		t.Error("lint-knowledge swallows its own failure")
	}
	if !strings.Contains(mk, "lint-all: ") || !strings.Contains(mk[strings.Index(mk, "lint-all: "):], "lint-knowledge") {
		t.Error("lint-all does not reach lint-knowledge")
	}
}

func TestCIRunsTheTargetAndDoesNotExcuseIt(t *testing.T) {
	ci := read(t, ".github/workflows/ci.yml")
	i := strings.Index(ci, "run: make lint-all")
	if i < 0 {
		t.Fatal("ci.yml never runs make lint-all, so the bundle is ungated on push")
	}
	// Only this step: gosec-sarif and the FOSSA scan are excused on purpose, and grading
	// every step in the file would make this test about somebody else's policy. A gate whose
	// failure is swallowed reports the same green as a passing one.
	step := ci[i:]
	if j := strings.Index(step, "\n      - "); j >= 0 {
		step = step[:j]
	}
	if strings.Contains(step, "continue-on-error") {
		t.Error("the lint-all step is continue-on-error, so a red bundle reports green")
	}
}

func TestTheHookIsExecutableInTheIndexNotOnlyOnDisk(t *testing.T) {
	root := repoRoot(t)
	out, err := exec.Command("git", "-C", root, "ls-files", "-s", ".githooks/pre-commit").Output()
	if err != nil || len(out) == 0 {
		t.Fatal(".githooks/pre-commit is not tracked, so a fresh clone gets no local gate")
	}
	// A hook chmod -x'd in the index is planted non-executable in every clone, and git
	// skips a hook it cannot execute without saying so.
	if mode := strings.Fields(string(out))[0]; mode != "100755" {
		t.Fatalf(".githooks/pre-commit is mode %s in the index, want 100755", mode)
	}
	if !strings.Contains(read(t, ".githooks/pre-commit"), "lint-all") {
		t.Error(".githooks/pre-commit does not run lint-all, so nothing local reaches the bundle")
	}
}

func TestThePinIsWrittenOnce(t *testing.T) {
	// An unpinned install lets this gate's verdict change with no commit in this repo.
	if !strings.Contains(read(t, "Makefile"), okfPin) {
		t.Fatalf("Makefile does not install %s", okfPin)
	}
}

func TestTheCheckerRejectsSomething(t *testing.T) {
	root := repoRoot(t)
	okf := filepath.Join(root, "bin", "okf")
	if _, err := os.Stat(okf); err != nil {
		found, err := exec.LookPath("okf")
		if err != nil {
			t.Fatalf("no okf in bin/ or on PATH, so lint-knowledge would fail every build: make tools")
		}
		okf = found
	}
	// Without this, everything above passes just as well against an okf that exits 0 on anything.
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, "broken.md"), []byte("---\ntitle: no type key\n---\n\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if exec.Command(okf, "check", bad).Run() == nil {
		t.Fatal("okf accepted a concept with no type key")
	}
}
