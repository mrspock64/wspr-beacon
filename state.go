package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type storedState struct {
	Config          BeaconConfig    `json:"config"`
	Schedule        []ScheduleEntry `json:"schedule"`
	ScheduleEnabled bool            `json:"scheduleEnabled"`
}

func (a *App) statePath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "WSPR Beacon", "state.json"), nil
}

func (a *App) loadState() {
	path, err := a.statePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state storedState
	if json.Unmarshal(data, &state) == nil && validateConfig(state.Config) == nil {
		a.config = state.Config
		if state.Schedule != nil {
			a.schedule = state.Schedule
		}
		a.scheduleEnabled = state.ScheduleEnabled
	}
}

func (a *App) saveStateLocked() error {
	path, err := a.statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(storedState{Config: a.config, Schedule: a.schedule, ScheduleEnabled: a.scheduleEnabled}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
