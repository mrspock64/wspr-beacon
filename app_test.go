package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildCommand(t *testing.T) {
	command := buildCommand(BeaconConfig{Callsign: "sm0abc", Grid: "jo89", Power: 23, Frequency: 14097100})
	if command != "CONFIG:SM0ABC,JO89,23,14097100" {
		t.Fatalf("unexpected command: %q", command)
	}
	command = buildCommand(BeaconConfig{Callsign: "SM0ABC", UseGPS: true, Power: 23, Frequency: 14097100})
	if !strings.Contains(command, "SM0ABC,    ,23") {
		t.Fatalf("GPS grid must use the firmware's four-space field: %q", command)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := BeaconConfig{Callsign: "SM0ABC", Grid: "JO89", Power: 23, Frequency: 14097100}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid configuration was rejected: %v", err)
	}
	invalid := valid
	invalid.Grid = "BAD"
	if err := validateConfig(invalid); err == nil {
		t.Fatal("invalid grid was accepted")
	}
	invalid = valid
	invalid.Frequency = 61000000
	if err := validateConfig(invalid); err == nil {
		t.Fatal("out-of-range frequency was accepted")
	}
}

func TestSafeConfigWindow(t *testing.T) {
	if !safeConfigWindow(time.Date(2026, 8, 18, 12, 1, 55, 0, time.Local)) {
		t.Fatal("end of odd minute should be safe")
	}
	if safeConfigWindow(time.Date(2026, 8, 18, 12, 0, 55, 0, time.Local)) {
		t.Fatal("even minute must not be safe")
	}
	if safeConfigWindow(time.Date(2026, 8, 18, 12, 1, 20, 0, time.Local)) {
		t.Fatal("early odd minute must not be safe")
	}
}

func TestParseObservedConfig(t *testing.T) {
	config, ok := parseObservedConfig("CFG:SM0ABC JO89 23 14095600")
	if !ok || config.Callsign != "SM0ABC" || config.Grid != "JO89" || config.Frequency != 14095600 {
		t.Fatalf("CFG line was not parsed: %#v, %v", config, ok)
	}
	config, ok = parseObservedConfig("OK SA0XYZ JO99 23 7040100")
	if !ok || config.Callsign != "SA0XYZ" || config.Frequency != 7040100 {
		t.Fatalf("OK line was not parsed: %#v, %v", config, ok)
	}
	if _, ok := parseObservedConfig("CFG:broken"); ok {
		t.Fatal("malformed CFG line was accepted")
	}
}
