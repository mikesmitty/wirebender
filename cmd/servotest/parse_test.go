package main

import (
	"math"
	"testing"
)

func TestParseM114_Normal(t *testing.T) {
	input := "LINEAR: 45.0mm BEND: 0.0 ROTATE: 0.0"
	result, err := ParseM114(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 axes, got %d", len(result))
	}
	if result["LINEAR"] != 45.0 {
		t.Errorf("FEED = %f, want 45.0", result["LINEAR"])
	}
	if result["BEND"] != 0.0 {
		t.Errorf("BEND = %f, want 0.0", result["BEND"])
	}
	if result["ROTATE"] != 0.0 {
		t.Errorf("ROTATE = %f, want 0.0", result["ROTATE"])
	}
}

func TestParseM114_Negative(t *testing.T) {
	input := "LINEAR: -45.0mm BEND: -30.0 ROTATE: 0.0"
	result, err := ParseM114(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["LINEAR"] != -45.0 {
		t.Errorf("FEED = %f, want -45.0", result["LINEAR"])
	}
	if result["BEND"] != -30.0 {
		t.Errorf("BEND = %f, want -30.0", result["BEND"])
	}
}

func TestParseM114_Fractional(t *testing.T) {
	input := "LINEAR: 22.5mm BEND: 0.0 ROTATE: 0.0"
	result, err := ParseM114(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(result["LINEAR"]-22.5) > 0.01 {
		t.Errorf("FEED = %f, want 22.5", result["LINEAR"])
	}
}

func TestParseM114_WithError(t *testing.T) {
	input := "LINEAR: 45.0mm BEND: ERROR (timeout) ROTATE: 0.0"
	result, err := ParseM114(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["BEND"]; ok {
		t.Error("BEND should be absent (ERROR)")
	}
	if result["LINEAR"] != 45.0 {
		t.Errorf("FEED = %f, want 45.0", result["LINEAR"])
	}
}

func TestParseM114_Empty(t *testing.T) {
	_, err := ParseM114("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseM122_Normal(t *testing.T) {
	lines := []string{
		"LINEAR: ID:1 Pos:45.0mm Raw:3072 Speed:0 Load:0 Volt:7V Temp:35C",
		"BEND: ID:2 Pos:0.0 Raw:2048 Speed:0 Load:0 Volt:7V Temp:34C",
		"ROTATE: ID:3 Pos:0.0 Raw:2048 Speed:0 Load:0 Volt:7V Temp:33C",
	}
	result, err := ParseM122(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 axes, got %d", len(result))
	}

	feed := result["LINEAR"]
	if feed.ID != 1 {
		t.Errorf("FEED ID = %d, want 1", feed.ID)
	}
	if feed.Pos != 45.0 {
		t.Errorf("FEED Pos = %f, want 45.0", feed.Pos)
	}
	if feed.Raw != 3072 {
		t.Errorf("FEED Raw = %d, want 3072", feed.Raw)
	}
	if feed.Voltage != 7 {
		t.Errorf("FEED Voltage = %d, want 7", feed.Voltage)
	}
	if feed.Temp != 35 {
		t.Errorf("FEED Temp = %d, want 35", feed.Temp)
	}
}

func TestParseM122_Negative(t *testing.T) {
	lines := []string{
		"LINEAR: ID:1 Pos:-45.0mm Raw:1024 Speed:-100 Load:-50 Volt:7V Temp:35C",
	}
	result, err := ParseM122(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	feed := result["LINEAR"]
	if feed.Pos != -45.0 {
		t.Errorf("FEED Pos = %f, want -45.0", feed.Pos)
	}
	if feed.Speed != -100 {
		t.Errorf("FEED Speed = %d, want -100", feed.Speed)
	}
	if feed.Load != -50 {
		t.Errorf("FEED Load = %d, want -50", feed.Load)
	}
}

func TestParseM122_Empty(t *testing.T) {
	_, err := ParseM122(nil)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseM122_Garbage(t *testing.T) {
	_, err := ParseM122([]string{"not a valid line"})
	if err == nil {
		t.Error("expected error for garbage input")
	}
}
