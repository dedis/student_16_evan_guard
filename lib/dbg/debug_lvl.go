// Debugging using different debug-levels for more or less verbose output.
// You can use
//	dbg.Lvl1("Important information")
//	dbg.Lvl2("Less important information")
//	dbg.Lvl3("Eventually flooding information")
//	dbg.Lvl4("Definitvely flooding information")
//	dbg.Lvl5("I hope you never need this")
// in your program, then according to the debug-level one or more levels of
// output will be shown. To set the debug-level, use
//	dbg.SetDebugVisible(3)
// which will show all `Lvl1`, `Lvl2`, and `Lvl3`. If you want to turn
// on just one output, you can use
//	dbg.LLvl2("Less important information")
// By adding a single 'L' to the method, it *always* gets printed.
package dbg

import (
	"flag"
	"fmt"
	"github.com/daviddengcn/go-colortext"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// These are information-debugging levels that can be turned on or off.
// Every logging greater than 'DebugVisible' will be discarded. So you can
// Log at different levels and easily turn on or off the amount of logging
// generated by adjusting the 'DebugVisible' variable.
var debugVisible = 1

// If showTime is true, it will print the time for each line of debug-output
var showTime = false

// If useColors is true, debug-output will be colored
var useColors = true

var debugMut sync.RWMutex

// The padding of functions to make a nice debug-output - this is automatically updated
// whenever there are longer functions and kept at that new maximum. If you prefer
// to have a fixed output and don't remember oversized names, put a negative value
// in here
var NamePadding = 40

// Padding of line-numbers for a nice debug-output - used in the same way as
// NamePadding
var LinePadding = 3

// Testing variable can have multiple values
// 0 - no testing
// 1 - put all line-numbers to 0
// 2 - like 1, but also write to TestString instead of stdout
var Testing = 0

var TestStr = ""

// If this variable is set, it will be outputted between the position and the message
var StaticMsg = ""

var regexpPaths, _ = regexp.Compile(".*/")

const (
	LvlPrint = iota - 10
	LvlWarning
	LvlError
	LvlFatal
	LvlPanic
)

func lvl(lvl int, args ...interface{}) {
	debugMut.Lock()
	defer debugMut.Unlock()

	if lvl > debugVisible {
		return
	}
	pc, _, line, _ := runtime.Caller(3)
	name := regexpPaths.ReplaceAllString(runtime.FuncForPC(pc).Name(), "")
	lineStr := fmt.Sprintf("%d", line)

	// For the testing-framework, we check the resulting string. So as not to
	// have the tests fail every time somebody moves the functions, we put
	// the line-# to 0
	if Testing > 0 {
		line = 0
	}

	if len(name) > NamePadding && NamePadding > 0 {
		NamePadding = len(name)
	}
	if len(lineStr) > LinePadding && LinePadding > 0 {
		LinePadding = len(name)
	}
	fmtstr := fmt.Sprintf("%%%ds: %%%dd", NamePadding, LinePadding)
	caller := fmt.Sprintf(fmtstr, name, line)
	if StaticMsg != "" {
		caller += "@" + StaticMsg
	}
	message := fmt.Sprintln(args...)
	bright := lvl < 0
	lvlAbs := lvl
	if bright {
		lvlAbs *= -1
	}
	lvlStr := strconv.Itoa(lvlAbs)
	if lvl < 0 {
		lvlStr += "!"
	}
	switch lvl {
	case LvlPrint:
		fg(ct.White, true)
		lvlStr = "I"
	case LvlWarning:
		fg(ct.Green, true)
		lvlStr = "W"
	case LvlError:
		fg(ct.Red, false)
		lvlStr = "E"
	case LvlFatal:
		fg(ct.Red, true)
		lvlStr = "F"
	case LvlPanic:
		fg(ct.Red, true)
		lvlStr = "P"
	default:
		if lvl != 0 {
			if lvlAbs <= 5 {
				colors := []ct.Color{ct.Yellow, ct.Cyan, ct.Green, ct.Blue, ct.Cyan}
				fg(colors[lvlAbs-1], bright)
			}
		}
	}
	str := fmt.Sprintf(": (%s) - %s", caller, message)
	if showTime {
		ti := time.Now()
		str = fmt.Sprintf("%s.%09d%s", ti.Format("06/02/01 15:04:05"), ti.Nanosecond(), str)
	}
	TestStr = fmt.Sprintf("%-2s%s", lvlStr, str)
	if Testing != 2 {
		fmt.Print(TestStr)
	}
	if useColors {
		ct.ResetColor()
	}
}

func fg(c ct.Color, bright bool) {
	if useColors {
		ct.Foreground(c, bright)
	}
}

// Needs two functions to keep the caller-depth the same and find who calls us
// Lvlf1 -> Lvlf -> lvl
// or
// Lvl1 -> lvld -> lvl
func lvlf(l int, f string, args ...interface{}) {
	lvl(l, fmt.Sprintf(f, args...))
}
func lvld(l int, args ...interface{}) {
	lvl(l, args...)
}

// Print directly sends the arguments to the stdout
func Print(args ...interface{}) {
	lvld(LvlPrint, args...)
}

// Printf is like Print but takes a formatting-argument first
func Printf(f string, args ...interface{}) {
	lvlf(LvlPrint, f, args...)
}

// Lvl1 debug output is informational and always displayed
func Lvl1(args ...interface{}) {
	lvld(1, args...)
}

// Lvl2 is more verbose but doesn't spam the stdout in case
// there is a big simulation
func Lvl2(args ...interface{}) {
	lvld(2, args...)
}

// Lvl3 gives debug-output that can make it difficult to read
// for big simulations with more than 100 hosts
func Lvl3(args ...interface{}) {
	lvld(3, args...)
}

// Lvl4 is only good for test-runs with very limited output
func Lvl4(args ...interface{}) {
	lvld(4, args...)
}

// Lvl5 is for big data
func Lvl5(args ...interface{}) {
	lvld(5, args...)
}

// Error prints the error in a nice red color
func Error(args ...interface{}) {
	lvld(LvlError, args...)
}

// Warn prints out the warning
func Warn(args ...interface{}) {
	lvld(LvlWarning, args...)
}

// Fatal prints out the fatal message and quits
func Fatal(args ...interface{}) {
	lvld(LvlFatal, args...)
	os.Exit(1)
}

// Panic prints out the panic message and panics
func Panic(args ...interface{}) {
	lvld(LvlPanic, args...)
	panic(args)
}

// Lvlf1 is like Lvl1 but with a format-string
func Lvlf1(f string, args ...interface{}) {
	lvlf(1, f, args...)
}

// Lvlf2 is like Lvl2 but with a format-string
func Lvlf2(f string, args ...interface{}) {
	lvlf(2, f, args...)
}

// Lvlf3 is like Lvl3 but with a format-string
func Lvlf3(f string, args ...interface{}) {
	lvlf(3, f, args...)
}

// Lvlf4 is like Lvl4 but with a format-string
func Lvlf4(f string, args ...interface{}) {
	lvlf(4, f, args...)
}

// Lvlf5 is like Lvl5 but with a format-string
func Lvlf5(f string, args ...interface{}) {
	lvlf(5, f, args...)
}

// Fatalf is like Fatal but with a format-string
func Fatalf(f string, args ...interface{}) {
	lvlf(LvlFatal, f, args...)
	os.Exit(1)
}

// Errorf is like Error but with a format-string
func Errorf(f string, args ...interface{}) {
	lvlf(LvlError, f, args...)
}

// Warnf is like Warn but with a format-string
func Warnf(f string, args ...interface{}) {
	lvlf(LvlWarning, f, args...)
}

// Panicf is like Panic but with a format-string
func Panicf(f string, args ...interface{}) {
	lvlf(LvlPanic, f, args...)
	panic(args)
}

// TestOutput sets the DebugVisible to 0 if 'show'
// is false, else it will set DebugVisible to 'level'
//
// Usage: TestOutput( test.Verbose(), 2 )
func TestOutput(show bool, level int) {
	debugMut.Lock()
	defer debugMut.Unlock()

	if show {
		debugVisible = level
	} else {
		debugVisible = 0
	}
}

// To easy print a debug-message anyway without discarding the level
// Just add an additional "L" in front, and remove it later:
// - easy hack to turn on other debug-messages
// - easy removable by searching/replacing 'LLvl' with 'Lvl'
// LLvl1 *always* prints
func LLvl1(args ...interface{}) { lvld(-1, args...) }

// LLvl2 *always* prints
func LLvl2(args ...interface{}) { lvld(-2, args...) }

// LLvl3 *always* prints
func LLvl3(args ...interface{}) { lvld(-3, args...) }

// LLvl4 *always* prints
func LLvl4(args ...interface{}) { lvld(-4, args...) }

// LLvl5 *always* prints
func LLvl5(args ...interface{}) { lvld(-5, args...) }

// LLvlf1 *always* prints
func LLvlf1(f string, args ...interface{}) { lvlf(-1, f, args...) }

// LLvlf2 *always* prints
func LLvlf2(f string, args ...interface{}) { lvlf(-2, f, args...) }

// LLvlf3 *always* prints
func LLvlf3(f string, args ...interface{}) { lvlf(-3, f, args...) }

// LLvlf4 *always* prints
func LLvlf4(f string, args ...interface{}) { lvlf(-4, f, args...) }

// LLvlf5 *always* prints
func LLvlf5(f string, args ...interface{}) { lvlf(-5, f, args...) }

// SetDebugVisible set the global debug output level in a go-rountine-safe way
func SetDebugVisible(lvl int) {
	debugMut.Lock()
	defer debugMut.Unlock()
	debugVisible = lvl
}

// DebugVisible returns the actual visible debug-level
func DebugVisible() int {
	debugMut.RLock()
	defer debugMut.RUnlock()
	return debugVisible
}

// SetShowTime allows for turning on the flag that adds the current
// time to the debug-output
func SetShowTime(show bool) {
	debugMut.Lock()
	defer debugMut.Unlock()
	showTime = show
}

// ShowTime returns the current setting for showing the time in the debug
// output
func ShowTime() bool {
	debugMut.Lock()
	defer debugMut.Unlock()
	return showTime
}

// SetUseColors can turn off or turn on the use of colors in the debug-output
func SetUseColors(show bool) {
	debugMut.Lock()
	defer debugMut.Unlock()
	useColors = show
}

// UseColors returns the actual setting of the color-usage in dbg
func UseColors() bool {
	debugMut.Lock()
	defer debugMut.Unlock()
	return useColors
}

// TestFatal calls t.Fatal in the case err != nil
func TestFatal(t *testing.T, err error, msg ...string) {
	if err != nil {
		lvld(LvlFatal, strings.Join(msg, " "), err)
		os.Exit(1)
	}
}

// ParseEnv looks at the following environment-variables:
// - DEBUG_LVL - for the actual debug-lvl - default is 1
// - DEBUG_TIME - whether to show the timestamp - default is false
// - DEBUG_COLOR - whether to color the output - default is true
func ParseEnv() {
	var err error
	dv := os.Getenv("DEBUG_LVL")
	if dv != "" {
		debugVisible, err = strconv.Atoi(dv)
		Lvl3("Setting level to", dv, debugVisible, err)
		if err != nil {
			Error("Couldn't convert", dv, "to debug-level")
		}
	}
	dt := os.Getenv("DEBUG_TIME")
	if dt != "" {
		showTime, err = strconv.ParseBool(dt)
		Lvl3("Setting showTime to", dt, showTime, err)
		if err != nil {
			Error("Couldn't convert", dt, "to boolean")
		}
	}
	dc := os.Getenv("DEBUG_COLOR")
	if dc != "" {
		useColors, err = strconv.ParseBool(dc)
		Lvl3("Setting useColor to", dc, showTime, err)
		if err != nil {
			Error("Couldn't convert", dc, "to boolean")
		}
	}
}

// AddFlags adds the flags and the variables for the debug-control
func AddFlags() {
	ParseEnv()
	flag.IntVar(&debugVisible, "debug", DebugVisible(), "Change debug level (0-5)")
	flag.BoolVar(&showTime, "debug-time", ShowTime(), "Shows the time of each message")
	flag.BoolVar(&useColors, "debug-color", UseColors(), "Colors each message")
}
