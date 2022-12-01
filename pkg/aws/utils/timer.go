package utils

import (
	"fmt"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/luyomo/OhMyTiUP/pkg/tui"
)

type ExecutionTimer struct {
	MessageTable [][]string
	StartTimer   time.Time
	LastTimer    time.Time
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
	c.MessageTable = append(c.MessageTable, []string{phase, strconv.Itoa(int(diff.Seconds()))})
	c.LastTimer = time.Now()
	return nil
}

// Message print

func (c *ExecutionTimer) Print() error {
	cyan := color.New(color.FgCyan, color.Bold)

	diff := (time.Now()).Sub(c.StartTimer)
	c.MessageTable = append(c.MessageTable, []string{"Total", strconv.Itoa(int(diff.Seconds()))})

	fmt.Printf("\n%s:\n", cyan.Sprint("Execution Time"))
	tui.PrintTable(c.MessageTable, true)

	return nil
}
