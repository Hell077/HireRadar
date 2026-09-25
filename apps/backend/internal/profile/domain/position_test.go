package domain

import "testing"

func TestNormalizePositions(t *testing.T) {
	titles, err := NormalizePositions([]string{" Go   Developer "})
	if err != nil || titles[0] != "Go Developer" {
		t.Fatalf("titles = %v, %v", titles, err)
	}
	if _, err := NormalizePositions([]string{"Go Developer", "go developer"}); err == nil {
		t.Fatal("duplicate accepted")
	}
}
