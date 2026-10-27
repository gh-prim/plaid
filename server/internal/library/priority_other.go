//go:build !linux && !darwin

package library

import "log/slog"

func lowerPriority(int, *slog.Logger) {}
