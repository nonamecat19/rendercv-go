package models

import "time"

type ValidationContext struct {
	InputFilePath string
	CurrentDate   any
}

func (c *ValidationContext) InputPath() (string, bool) {
	if c == nil || c.InputFilePath == "" {
		return "", false
	}
	return c.InputFilePath, true
}

func (c *ValidationContext) Today() time.Time {
	if c == nil {
		return time.Now()
	}
	if d, ok := c.CurrentDate.(time.Time); ok {
		return d
	}
	if s, ok := c.CurrentDate.(string); ok && s == "today" {
		return time.Now()
	}
	return time.Now()
}
