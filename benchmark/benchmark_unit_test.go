package benchmark

import (
	"testing"
)

func TestSplitSentences_ThreeSentences_SplitsAsExpected(t *testing.T) {
	input := "Hello world. How are you? I am fine!"
	sentences := splitSentences(input)
	if len(sentences) != 3 {
		t.Fatalf("expected 3 sentences, got %d", len(sentences))
	}
	if sentences[0] != "Hello world." {
		t.Errorf("expected 'Hello world.', got '%s'", sentences[0])
	}
	if sentences[1] != "How are you?" {
		t.Errorf("expected 'How are you?', got '%s'", sentences[1])
	}
	if sentences[2] != "I am fine!" {
		t.Errorf("expected 'I am fine!', got '%s'", sentences[2])
	}
}

func TestMean_Inputs_ReturnExpectedValues(t *testing.T) {
	scores := []float64{1.0, 2.0, 3.0}
	if mean(scores) != 2.0 {
		t.Error("expected 2.0")
	}
	if mean(nil) != 0.0 {
		t.Error("expected 0.0")
	}
}

func TestStdDev_Sample_ReturnsNonZero(t *testing.T) {
	scores := []float64{2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0}
	if stdDev(scores) == 0.0 {
		t.Error("expected non-zero stdDev")
	}
}

func TestStdDev_SingleValue_ReturnsZero(t *testing.T) {
	if stdDev([]float64{1.0}) != 0.0 {
		t.Error("expected 0.0 for single item")
	}
}

func TestPercentile_P50_ReturnsMedian(t *testing.T) {
	latencies := []int64{10, 20, 30, 40, 50}
	p := percentile(latencies, 50)
	if p != 30 {
		t.Errorf("expected 30, got %d", p)
	}
}
