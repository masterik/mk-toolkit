package repoconfig

import (
	"strings"
	"testing"
)

// Issue #18's two new keys, held to #19's rules: an enum goes through the same
// vocabulary machinery, and a value that cannot mean anything is reported and
// cleared rather than handed on.

func TestSubjectMaxIsPinnedWhenItIsAUsableLength(t *testing.T) {
	c := loadFile(t, "[commit]\nsubject_max = 72\n")
	if len(c.Problems) != 0 {
		t.Fatalf("a usable length was reported: %+v", c.Problems)
	}
	if c.Commit.SubjectMax == nil || *c.Commit.SubjectMax != 72 {
		t.Errorf("subject_max = %v, want 72", c.Commit.SubjectMax)
	}
}

// Absent is nil, not zero. The pointer exists so that the next case can be told
// apart from this one at all.
func TestSubjectMaxAbsentIsNilAndSilent(t *testing.T) {
	c := loadFile(t, "[commit]\nscopes = [\"cli\"]\n")
	if c.Commit.SubjectMax != nil {
		t.Errorf("an absent subject_max decoded to %d", *c.Commit.SubjectMax)
	}
	if len(c.Problems) != 0 {
		t.Errorf("absent reported a problem: %+v", c.Problems)
	}
}

// Zero is not "no limit" — it is a pin that would take no effect, which is the
// one thing #19's machinery exists to make visible. Negative is the same answer.
func TestSubjectMaxRejectsAValueThatIsNotALength(t *testing.T) {
	for _, n := range []string{"0", "-10"} {
		c := loadFile(t, "[commit]\nsubject_max = "+n+"\n")
		pb := c.Problem("commit.subject_max")
		if pb == nil {
			t.Fatalf("subject_max = %s was accepted: %+v", n, c.Problems)
		}
		if pb.Kind != ProblemInvalidValue || pb.Value != n {
			t.Errorf("subject_max = %s: got %+v", n, *pb)
		}
		// The sentence has to say what happens next, and what happens next is
		// not discovery — there is none for this key.
		if !strings.Contains(pb.Detail, "positive number of characters") ||
			!strings.Contains(pb.Detail, pb.Path) {
			t.Errorf("subject_max = %s: detail is not actionable: %s", n, pb.Detail)
		}
		if strings.Contains(pb.Detail, "discovery answers instead") {
			t.Errorf("subject_max = %s: detail promises a discovered answer that does not exist: %s", n, pb.Detail)
		}
		if c.Commit.SubjectMax != nil {
			t.Errorf("subject_max = %s survived as %d", n, *c.Commit.SubjectMax)
		}
	}
}

// A large value is silly, not wrong. Refusing it would pin a house style into
// the schema, which is a different job from catching a mistake.
func TestSubjectMaxHasNoUpperBound(t *testing.T) {
	if c := loadFile(t, "[commit]\nsubject_max = 500\n"); len(c.Problems) != 0 {
		t.Errorf("500 was refused: %+v", c.Problems)
	}
}

func TestReviewModeGoesThroughTheSameVocabularyAsMergeStyle(t *testing.T) {
	for _, m := range ReviewModes {
		c := loadFile(t, "[review]\nmode = \""+m+"\"\n")
		if c.Review.Mode != m || len(c.Problems) != 0 {
			t.Errorf("review.mode %q rejected: %+v", m, c.Problems)
		}
	}
	c := loadFile(t, "[review]\nmode = \"fast\"\n")
	pb := c.Problem("review.mode")
	if pb == nil || pb.Kind != ProblemInvalidValue {
		t.Fatalf("review.mode = \"fast\" was accepted: %+v", c.Problems)
	}
	for _, a := range Allowed("review.mode") {
		if !strings.Contains(pb.Detail, a) {
			t.Errorf("detail does not name allowed value %q: %s", a, pb.Detail)
		}
	}
	// Cleared, so no skill branches on "fast".
	if c.Review.Mode != "" {
		t.Errorf("invalid mode survived as %q", c.Review.Mode)
	}
}

// The round trip, as TestWrittenConfigLoadsWithNoProblems does for the older
// keys: the file is rendered from a template, so only a test proves the strict
// reader still accepts what the writer emits for the new ones.
func TestWrittenCommitAndReviewKeysRoundTrip(t *testing.T) {
	dir := t.TempDir()
	n := 72
	if err := Write(dir, &Config{
		Commit: Commit{Scopes: []string{"cli"}, SubjectMax: &n},
		Review: Review{Reviewers: []string{"@a"}, Mode: "quick"},
	}); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Problems) != 0 {
		t.Fatalf("the file mkit writes does not load cleanly: %+v", c.Problems)
	}
	if c.Commit.SubjectMax == nil || *c.Commit.SubjectMax != n || c.Review.Mode != "quick" {
		t.Errorf("round trip lost values: %+v", c)
	}
}

// Either key alone still renders its table. A `[commit]` block emitted only when
// scopes are set would drop a subject_max pinned on its own.
func TestEitherKeyAloneStillWritesItsTable(t *testing.T) {
	for _, c := range []*Config{
		{Commit: Commit{SubjectMax: intp(50)}},
		{Review: Review{Mode: "full"}},
	} {
		dir := t.TempDir()
		if err := Write(dir, c); err != nil {
			t.Fatal(err)
		}
		got, _, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got.IsZero() {
			t.Errorf("a config pinning only one of the new keys round-tripped to nothing: %+v", c)
		}
	}
}

func TestIsZeroCountsTheNewKeys(t *testing.T) {
	if (&Config{Commit: Commit{SubjectMax: intp(72)}}).IsZero() {
		t.Error("a pinned subject_max reads as nothing pinned, so `mkit init` would call it empty")
	}
	if (&Config{Review: Review{Mode: "quick"}}).IsZero() {
		t.Error("a pinned review mode reads as nothing pinned")
	}
}

func intp(n int) *int { return &n }
