package models

import "time"

// ValidationContext is the context threaded to validators, mirroring
// schema/models/validation_context.py.
type ValidationContext struct {
	InputFilePath string
	CurrentDate   any
}

// InputPath reports the input file path and whether one was supplied.
func (c *ValidationContext) InputPath() (string, bool) {
	if c == nil || c.InputFilePath == "" {
		return "", false
	}
	return c.InputFilePath, true
}

// Today reports the reference date: the context's current date when it is a
// real date, and today otherwise — an unparseable value falls back silently.
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
