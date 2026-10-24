package translation

import (
	"strings"
	"testing"
)

func TestDecodePreservesSourceAndRejectsIncompleteOutput(t *testing.T) {
	source := []Segment{{ID: "0", Text: "Benchmark improves by 25%."}, {ID: "1", Text: "Keep the code."}}
	good := []byte(`{"segments":[{"id":"0","translation":"基准测试提升了 25%。"},{"id":"1","translation":"保留代码。"}]}`)
	result, err := Decode(good, source)
	if err != nil || result[0].Text != source[0].Text || result[1].Translation != "保留代码。" {
		t.Fatalf("bad result: %v, %v", result, err)
	}
	for _, bad := range []string{
		`{"segments":[{"id":"0","translation":"不完整"}]}`,
		`{"segments":[{"id":"1","translation":"错位"},{"id":"0","translation":"错位"}]}`,
		`{"segments":[{"id":"0","translation":" "},{"id":"1","translation":"译文"}]}`,
	} {
		if _, err := Decode([]byte(bad), source); err == nil {
			t.Errorf("accepted invalid output: %s", bad)
		}
	}
}

func TestBatchNeverSplitsOrDropsAParagraph(t *testing.T) {
	segments := []Segment{{ID: "0", Text: strings.Repeat("a", 8000)}, {ID: "1", Text: strings.Repeat("中", 2000)}, {ID: "2", Text: "End"}}
	first := Batch(segments)
	if len(first) != 1 {
		t.Fatalf("expected first paragraph only, got %d", len(first))
	}
	second := Batch(segments[len(first):])
	if len(second) != 2 || second[1].ID != "2" {
		t.Fatalf("lost remaining paragraphs: %v", second)
	}
	oversized := []Segment{{ID: "long", Text: strings.Repeat("a", 10000)}}
	if len(Batch(oversized)) != 1 {
		t.Fatal("oversized paragraph must still advance the queue")
	}
}
