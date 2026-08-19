package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.bug.st/serial"
)

var callsignPattern = regexp.MustCompile(`^[A-Z0-9]{1,6}$`)
var gridPattern = regexp.MustCompile(`^[A-R]{2}[0-9]{2}$`)
var timePattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

var supportedBands = map[string]int64{
	"160 m": 1838100, "80 m": 3570100, "40 m": 7040100, "30 m": 10140200,
	"20 m": 14097100, "17 m": 18106100, "15 m": 21096100, "12 m": 24926100,
	"10 m": 28126100, "6 m": 50294500,
}

type BeaconConfig struct {
	Callsign  string `json:"callsign"`
	Grid      string `json:"grid"`
	UseGPS    bool   `json:"useGPS"`
	Power     int    `json:"power"`
	Frequency int64  `json:"frequency"`
}
type ScheduleEntry struct {
	ID        string `json:"id"`
	Days      []int  `json:"days"`
	Start     string `json:"start"`
	Band      string `json:"band"`
	Frequency int64  `json:"frequency"`
	Enabled   bool   `json:"enabled"`
}
type BeaconStatus struct {
	Connected        bool   `json:"connected"`
	Simulated        bool   `json:"simulated"`
	Port             string `json:"port"`
	GPSLocked        bool   `json:"gpsLocked"`
	Transmitting     bool   `json:"transmitting"`
	LastTransmission string `json:"lastTransmission"`
	NextWindow       string `json:"nextWindow"`
	Message          string `json:"message"`
	ScheduleEnabled  bool   `json:"scheduleEnabled"`
}

type App struct {
	ctx             context.Context
	mu              sync.RWMutex
	port            serial.Port
	portName        string
	simulated       bool
	config          BeaconConfig
	detectedConfig  *BeaconConfig
	status          BeaconStatus
	schedule        []ScheduleEntry
	scheduleEnabled bool
	logs            []string
	cancel          context.CancelFunc
}

func NewApp() *App {
	a := &App{config: BeaconConfig{Callsign: "SM0ABC", Grid: "JO89", Power: 23, Frequency: 14097100}, status: BeaconStatus{Message: "Inte ansluten"}, schedule: []ScheduleEntry{
		{ID: "morning-40", Days: []int{1, 2, 3, 4, 5}, Start: "06:00", Band: "40 m", Frequency: 7040100, Enabled: true},
		{ID: "day-20", Days: []int{0, 1, 2, 3, 4, 5, 6}, Start: "09:00", Band: "20 m", Frequency: 14097100, Enabled: true},
		{ID: "evening-40", Days: []int{0, 1, 2, 3, 4, 5, 6}, Start: "19:00", Band: "40 m", Frequency: 7040100, Enabled: true},
	}}
	a.loadState()
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.addLog("WSPR Beacon redo. Välj serieport eller starta simulerat läge.")
	schedulerCtx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go a.scheduler(schedulerCtx)
}
func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	a.Disconnect()
}

func (a *App) ListPorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	sort.Strings(ports)
	return ports, nil
}
func (a *App) Connect(portName string, simulated bool) (BeaconStatus, error) {
	a.Disconnect()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.simulated = simulated
	a.portName = portName
	if simulated {
		a.status = BeaconStatus{Connected: true, Simulated: true, Port: "Simulerad beacon", GPSLocked: true, Message: "Simulerat läge - ingen radio styrs"}
		a.addLogLocked("Simulerat läge anslutet.")
		return a.status, nil
	}
	if portName == "" {
		return a.status, errors.New("välj en serieport")
	}
	p, err := serial.Open(portName, &serial.Mode{BaudRate: 9600})
	if err != nil {
		return a.status, fmt.Errorf("kunde inte öppna %s: %w", portName, err)
	}
	a.port = p
	a.status = BeaconStatus{Connected: true, Port: portName, Message: "Ansluten - väntar på beaconstatus"}
	a.addLogLocked("Ansluten till " + portName + " vid 9600 baud.")
	go a.readSerial(p)
	return a.status, nil
}
func (a *App) Disconnect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.port != nil {
		_ = a.port.Close()
		a.port = nil
	}
	if a.status.Connected {
		a.addLogLocked("Anslutning stängd.")
	}
	a.status = BeaconStatus{Message: "Inte ansluten"}
	a.portName = ""
	a.simulated = false
}
func (a *App) GetStatus() BeaconStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.refreshTimingLocked(time.Now())
	a.status.ScheduleEnabled = a.scheduleEnabled
	return a.status
}
func (a *App) GetConfig() BeaconConfig { a.mu.RLock(); defer a.mu.RUnlock(); return a.config }
func (a *App) GetLogs() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]string(nil), a.logs...)
}
func (a *App) GetSchedule() []ScheduleEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]ScheduleEntry(nil), a.schedule...)
}

func (a *App) GetScheduleEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.scheduleEnabled
}

func (a *App) SetScheduleEnabled(enabled bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.scheduleEnabled = enabled
	a.status.ScheduleEnabled = enabled
	if err := a.saveStateLocked(); err != nil {
		return fmt.Errorf("kunde inte spara driftläge: %w", err)
	}
	if enabled {
		a.addLogLocked("Schemaläge aktiverat.")
	} else {
		a.addLogLocked("Schemaläge avaktiverat - manuell konfiguration är aktiv.")
	}
	return nil
}

// LoadDetectedConfig loads the most recent CFG/OK configuration observed on the serial port.
// This firmware documents no read command, so the beacon must emit CFG at startup or OK after a config write.
func (a *App) LoadDetectedConfig() (BeaconConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.simulated {
		a.detectedConfig = &a.config
	}
	if a.detectedConfig == nil {
		return BeaconConfig{}, errors.New("ingen CFG-rad har mottagits ännu; koppla från och anslut beaconen igen eller starta om den medan loggen är öppen")
	}
	a.config = *a.detectedConfig
	if err := a.saveStateLocked(); err != nil {
		return BeaconConfig{}, fmt.Errorf("konfigurationen lästes men kunde inte sparas lokalt: %w", err)
	}
	a.addLogLocked("Senast mottagna enhetskonfiguration laddad i formuläret.")
	return a.config, nil
}

func (a *App) SaveSchedule(entries []ScheduleEntry) error {
	for _, entry := range entries {
		if _, ok := supportedBands[entry.Band]; !ok {
			return fmt.Errorf("okänt band: %s", entry.Band)
		}
		if !timePattern.MatchString(entry.Start) {
			return fmt.Errorf("ogiltig starttid: %s", entry.Start)
		}
		if len(entry.Days) == 0 {
			return errors.New("varje schemarad måste ha minst en veckodag")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.schedule = entries
	if err := a.saveStateLocked(); err != nil {
		return fmt.Errorf("schemat uppdaterades men kunde inte sparas lokalt: %w", err)
	}
	a.addLogLocked("Schema sparat lokalt.")
	return nil
}

func (a *App) ApplyConfig(config BeaconConfig) (string, error) {
	if err := validateConfig(config); err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.status.Connected {
		return "", errors.New("anslut beaconen först")
	}
	command := buildCommand(config)
	if a.simulated {
		a.config = config
		if err := a.saveStateLocked(); err != nil {
			return "", fmt.Errorf("konfigurationen tillämpades men kunde inte sparas lokalt: %w", err)
		}
		response := fmt.Sprintf("OK %s %s %d %d", config.Callsign, gridForResponse(config), config.Power, config.Frequency)
		a.addLogLocked("> " + command)
		a.addLogLocked("< " + response)
		return response, nil
	}
	if a.port == nil {
		return "", errors.New("serieporten är inte tillgänglig")
	}
	if _, err := a.port.Write([]byte(command + "\n")); err != nil {
		return "", fmt.Errorf("kunde inte skicka konfiguration: %w", err)
	}
	a.config = config
	if err := a.saveStateLocked(); err != nil {
		return "", fmt.Errorf("konfigurationen skickades men kunde inte sparas lokalt: %w", err)
	}
	a.addLogLocked("> " + command)
	return "Konfiguration skickad - inväntar OK från beaconen", nil
}

func validateConfig(c BeaconConfig) error {
	c.Callsign = strings.ToUpper(strings.TrimSpace(c.Callsign))
	if !callsignPattern.MatchString(c.Callsign) {
		return errors.New("anropssignal måste vara 1-6 bokstäver eller siffror")
	}
	if !c.UseGPS && !gridPattern.MatchString(strings.ToUpper(strings.TrimSpace(c.Grid))) {
		return errors.New("grid måste vara fyra tecken, till exempel JO89")
	}
	if c.Power < 0 || c.Power > 60 {
		return errors.New("effekt måste vara 0-60 dBm")
	}
	if c.Frequency < 1000000 || c.Frequency > 60000000 {
		return errors.New("frekvens måste vara mellan 1 och 60 MHz")
	}
	return nil
}
func buildCommand(c BeaconConfig) string {
	grid := strings.ToUpper(strings.TrimSpace(c.Grid))
	if c.UseGPS {
		grid = "    "
	}
	return fmt.Sprintf("CONFIG:%s,%s,%d,%d", strings.ToUpper(strings.TrimSpace(c.Callsign)), grid, c.Power, c.Frequency)
}
func gridForResponse(c BeaconConfig) string {
	if c.UseGPS {
		return "GPS"
	}
	return strings.ToUpper(c.Grid)
}
func (a *App) readSerial(p serial.Port) {
	scanner := bufio.NewScanner(p)
	for scanner.Scan() {
		a.handleSerialLine(strings.TrimSpace(scanner.Text()))
	}
}
func (a *App) handleSerialLine(line string) {
	if line == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addLogLocked("< " + line)
	upper := strings.ToUpper(line)
	if strings.HasPrefix(upper, "TX:") {
		a.status.LastTransmission = time.Now().Format("15:04:05")
		a.status.Transmitting = false
	}
	if strings.Contains(upper, "GPS") && (strings.Contains(upper, "LOCK") || strings.Contains(upper, "SYNC")) {
		a.status.GPSLocked = true
	}
	if strings.HasPrefix(upper, "ERR") {
		a.status.Message = "Beaconen avvisade konfigurationen"
	}
	if strings.HasPrefix(upper, "OK ") {
		a.status.Message = "Konfiguration bekräftad av beaconen"
	}
	if config, ok := parseObservedConfig(line); ok {
		a.detectedConfig = &config
		a.status.Message = "Enhetskonfiguration mottagen - redo att laddas manuellt"
	}
}

func parseObservedConfig(line string) (BeaconConfig, bool) {
	parts := strings.Fields(line)
	if len(parts) == 0 || (!strings.HasPrefix(strings.ToUpper(parts[0]), "CFG:") && strings.ToUpper(parts[0]) != "OK") {
		return BeaconConfig{}, false
	}
	if strings.HasPrefix(strings.ToUpper(parts[0]), "CFG:") {
		if len(parts) != 4 {
			return BeaconConfig{}, false
		}
		parts[0] = strings.TrimPrefix(strings.ToUpper(parts[0]), "CFG:")
		if parts[0] == "" {
			return BeaconConfig{}, false
		}
	} else {
		if len(parts) != 5 {
			return BeaconConfig{}, false
		}
		parts = parts[1:]
	}
	if len(parts) != 4 {
		return BeaconConfig{}, false
	}
	var power int
	var frequency int64
	if _, err := fmt.Sscanf(parts[2], "%d", &power); err != nil {
		return BeaconConfig{}, false
	}
	if _, err := fmt.Sscanf(parts[3], "%d", &frequency); err != nil {
		return BeaconConfig{}, false
	}
	config := BeaconConfig{Callsign: strings.ToUpper(parts[0]), Grid: strings.ToUpper(parts[1]), Power: power, Frequency: frequency}
	return config, validateConfig(config) == nil
}

func (a *App) scheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.mu.Lock()
			a.refreshTimingLocked(now)
			entry := a.activeScheduleLocked(now)
			shouldApply := a.scheduleEnabled && entry != nil && a.status.Connected && !a.status.Transmitting && a.config.Frequency != entry.Frequency && safeConfigWindow(now)
			var desired BeaconConfig
			if shouldApply {
				desired = a.config
				desired.Frequency = entry.Frequency
			}
			a.mu.Unlock()
			if shouldApply {
				if _, err := a.ApplyConfig(desired); err != nil {
					a.addLog("Schemat kunde inte tillämpas: " + err.Error())
				}
			}
		}
	}
}
func safeConfigWindow(now time.Time) bool { return now.Minute()%2 == 1 && now.Second() >= 52 }
func (a *App) activeScheduleLocked(now time.Time) *ScheduleEntry {
	day := int(now.Weekday())
	var selected *ScheduleEntry
	for i := range a.schedule {
		e := &a.schedule[i]
		if !e.Enabled || !containsDay(e.Days, day) || e.Start > now.Format("15:04") {
			continue
		}
		if selected == nil || e.Start > selected.Start {
			selected = e
		}
	}
	return selected
}
func containsDay(days []int, target int) bool {
	for _, day := range days {
		if day == target {
			return true
		}
	}
	return false
}
func (a *App) refreshTimingLocked(now time.Time) {
	next := now.Truncate(2 * time.Minute).Add(2 * time.Minute)
	if now.Minute()%2 == 1 {
		next = now.Truncate(time.Minute).Add(time.Minute)
	}
	a.status.NextWindow = next.Format("15:04")
	// The firmware only emits TX:<...> DONE, not a TX-start signal. Do not infer
	// a locked state from the clock: a manual CONFIG can safely be queued by the
	// beacon while it is transmitting. The scheduler itself still uses its safe window.
	a.status.Transmitting = false
}
func (a *App) addLog(line string) { a.mu.Lock(); defer a.mu.Unlock(); a.addLogLocked(line) }
func (a *App) addLogLocked(line string) {
	entry := fmt.Sprintf("%s  %s", time.Now().Format("15:04:05"), line)
	a.logs = append(a.logs, entry)
	if len(a.logs) > 250 {
		a.logs = a.logs[len(a.logs)-250:]
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "beacon:log", entry)
	}
}
