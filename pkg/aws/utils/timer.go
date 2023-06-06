package utils

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
)

type ExecutionTimer struct {
	MessageTable [][]string
	StartTimer   time.Time
	LastTimer    time.Time
}

func FormatDuration(d time.Duration) string {
	scale := 100 * time.Second
	// look for the max scale that is smaller than d
	for scale > d {
		scale = scale / 10
	}
	return d.Round(scale / 100).String()
}

func NewTimer() *ExecutionTimer {
	var ins ExecutionTimer
	ins.StartTimer = time.Now()
	ins.LastTimer = ins.StartTimer

	return &ins
}

func (c *ExecutionTimer) Initialize(header []string) error {
	c.MessageTable = append(c.MessageTable, header)
	c.StartTimer = time.Now()
	c.LastTimer = c.StartTimer
	return nil
}

// Timer handle
func (c *ExecutionTimer) Take(phase string) error {
	diff := (time.Now()).Sub(c.LastTimer)

	// c.MessageTable = append(c.MessageTable, []string{phase, diff.String()})
	c.MessageTable = append(c.MessageTable, []string{phase, FormatDuration(diff)})
	c.LastTimer = time.Now()
	return nil
}

func (c *ExecutionTimer) Append(copyTimer *ExecutionTimer) error {
	for _, message := range (*copyTimer).MessageTable {
		c.MessageTable = append(c.MessageTable, message)
	}

	return nil
}

// Message print

func (c *ExecutionTimer) Print() error {
	cyan := color.New(color.FgCyan, color.Bold)

	diff := (time.Now()).Sub(c.StartTimer)

	c.MessageTable = append(c.MessageTable, []string{"Total", FormatDuration(diff)})

	fmt.Printf("\n%s:\n", cyan.Sprint("Execution Time"))
	tui.PrintTable(c.MessageTable, true)

	return nil
}
