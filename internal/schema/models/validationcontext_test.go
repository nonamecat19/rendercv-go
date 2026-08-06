package models_test

import (
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models"
)

func TestValidationContextNil(t *testing.T) {
	var c *models.ValidationContext
	path, ok := c.InputPath()
	if ok || path != "" {
		t.Errorf("nil context InputPath() = (%q, %v), want (\"\", false)", path, ok)
	}
	today := c.Today()
	if today.IsZero() {
		t.Error("nil context Today() returned zero time")
	}
}

func TestValidationContextEmpty(t *testing.T) {
	c := &models.ValidationContext{}
	path, ok := c.InputPath()
	if ok || path != "" {
		t.Errorf("empty context InputPath() = (%q, %v), want (\"\", false)", path, ok)
	}
}

func TestValidationContextInputPath(t *testing.T) {
	c := &models.ValidationContext{InputFilePath: "/test/file.yaml"}
	path, ok := c.InputPath()
	if !ok || path != "/test/file.yaml" {
		t.Errorf("InputPath() = (%q, %v), want (/test/file.yaml, true)", path, ok)
	}
}

func TestValidationContextTodayLiteral(t *testing.T) {
	c := &models.ValidationContext{CurrentDate: "today"}
	today := c.Today()
	expected := time.Now()
	if today.Year() != expected.Year() || today.Month() != expected.Month() || today.Day() != expected.Day() {
		t.Errorf("Today() with 'today' = %v, want today", today)
	}
}

func TestValidationContextTodayRealDate(t *testing.T) {
	d := time.Date(2008, 1, 15, 0, 0, 0, 0, time.UTC)
	c := &models.ValidationContext{CurrentDate: d}
	got := c.Today()
	if !got.Equal(d) {
		t.Errorf("Today() with real date = %v, want %v", got, d)
	}
}

func TestValidationContextTodayFallback(t *testing.T) {
	c := &models.ValidationContext{CurrentDate: "yesterday"}
	today := c.Today()
	expected := time.Now()
	if today.Year() != expected.Year() || today.Month() != expected.Month() || today.Day() != expected.Day() {
		t.Errorf("Today() with 'yesterday' = %v, want today", today)
	}
}
