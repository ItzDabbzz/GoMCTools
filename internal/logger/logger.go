package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/log"
)

var Log *log.Logger
var logFile *os.File
var out io.Writer = os.Stderr

// GetConfigPath returns the absolute path to the TOML config file,
// creating the parent directory if it does not yet exist.
func GetLoggerPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "gomctools")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create config directory: %w", err)
	}

	return filepath.Join(configDir, "logger.log"), nil
}

func InitLogger() {
	debug := os.Getenv("DEBUG") != ""
	dev := os.Getenv("DEV") != ""

	path, err := GetLoggerPath()
	if err != nil {
		panic(err)
	}

	f, err := tea.LogToFile(path, "log")
	if err != nil {
		panic(err)
	}

	logFile = f
	out = f
	if dev {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			panic(err)
		}

		logFile = f
		out = f
	}

	Log = log.NewWithOptions(out, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		CallerOffset:    1,
		Formatter:       log.JSONFormatter,
	})

	if debug {
		Log.SetLevel(log.DebugLevel)
	} else {
		Log.SetLevel(log.ErrorLevel)
	}
}

func CloseLogger() {
	if logFile != nil {
		err := logFile.Close()
		if err != nil {
			panic(err)
		}
	}
}

func Debug(msg string, keyvals ...any) {
	if len(msg) > 0 {
		Log.Debug(msg, keyvals...)
	}
}

func Info(msg string, keyvals ...any) {
	if len(msg) > 0 {
		Log.Info(msg, keyvals...)
	}
}

func Warn(msg string, keyvals ...any) {
	if len(msg) > 0 {
		Log.Warn(msg, keyvals...)
	}
}

func Error(msg string, keyvals ...any) {
	if len(msg) > 0 {
		Log.Error(msg, keyvals...)
	}
}

func Fatal(msg string, keyvals ...any) {
	if len(msg) > 0 {
		Log.Fatal(msg, keyvals...)
	}
}

func Print(msg string, keyvals ...any) {
	if len(msg) > 0 {
		Log.Print(msg, keyvals...)
	}
}

func Debugf(format string, args ...any) {
	Log.Debugf(format, args...)
}

func Infof(format string, args ...any) {
	Log.Infof(format, args...)
}

func Warnf(format string, args ...any) {
	Log.Warnf(format, args...)
}

func Errorf(format string, args ...any) {
	Log.Errorf(format, args...)
}

func Fatalf(format string, args ...any) {
	Log.Fatalf(format, args...)
}

func Printf(format string, args ...any) {
	Log.Printf(format, args...)
}
