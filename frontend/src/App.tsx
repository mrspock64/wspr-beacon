import { useCallback, useEffect, useMemo, useState } from "react";
import "./App.css";

type Config = {
  callsign: string;
  grid: string;
  useGPS: boolean;
  power: number;
  frequency: number;
};
type Status = {
  connected: boolean;
  simulated: boolean;
  port: string;
  gpsLocked: boolean;
  transmitting: boolean;
  lastTransmission: string;
  nextWindow: string;
  message: string;
  scheduleEnabled: boolean;
};
type Entry = {
  id: string;
  days: number[];
  start: string;
  band: string;
  frequency: number;
  enabled: boolean;
};
type Backend = {
  GetStatus: () => Promise<Status>;
  GetConfig: () => Promise<Config>;
  GetLogs: () => Promise<string[]>;
  GetSchedule: () => Promise<Entry[]>;
  GetScheduleEnabled: () => Promise<boolean>;
  SetScheduleEnabled: (enabled: boolean) => Promise<void>;
  LoadDetectedConfig: () => Promise<Config>;
  ListPorts: () => Promise<string[]>;
  Connect: (port: string, simulated: boolean) => Promise<Status>;
  Disconnect: () => Promise<void>;
  ApplyConfig: (config: Config) => Promise<string>;
  SaveSchedule: (entries: Entry[]) => Promise<void>;
};
declare global {
  interface Window {
    go?: { main?: { App?: Backend } };
  }
}
const BANDS = [
  ["160 m", 1838100],
  ["80 m", 3570100],
  ["40 m", 7040100],
  ["30 m", 10140200],
  ["20 m", 14097100],
  ["17 m", 18106100],
  ["15 m", 21096100],
  ["12 m", 24926100],
  ["10 m", 28126100],
  ["6 m", 50294500],
] as const;
const DAY_NAMES = ["Sön", "Mån", "Tis", "Ons", "Tor", "Fre", "Lör"];
const api = () => window.go?.main?.App;
const frequencyLabel = (hz: number) =>
  (hz / 1_000_000).toLocaleString("sv-SE", {
    minimumFractionDigits: 4,
    maximumFractionDigits: 4,
  });
const bandFor = (frequency: number) =>
  BANDS.find(([, value]) => value === frequency)?.[0] ?? "Eget band";
function Icon({
  name,
}: {
  name:
    | "radio"
    | "settings"
    | "calendar"
    | "log"
    | "gps"
    | "send"
    | "refresh"
    | "alert";
}) {
  const paths: Record<string, React.ReactNode> = {
    radio: (
      <>
        <circle cx="12" cy="12" r="2" />
        <path d="M7 7a7 7 0 0 0 0 10M17 7a7 7 0 0 1 0 10M4 4a11 11 0 0 0 0 16M20 4a11 11 0 0 1 0 16" />
      </>
    ),
    settings: (
      <>
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.12 2.12-.06-.06a1.7 1.7 0 0 0-1.88-.34 1.7 1.7 0 0 0-1.04 1.56v.08h-3v-.08A1.7 1.7 0 0 0 10.66 18.7a1.7 1.7 0 0 0-1.88.34l-.06.06L6.6 16.98l.06-.06A1.7 1.7 0 0 0 7 15.04a1.7 1.7 0 0 0-1.56-1.04h-.08v-3h.08A1.7 1.7 0 0 0 7 9.96a1.7 1.7 0 0 0-.34-1.88L6.6 8.02 8.72 5.9l.06.06a1.7 1.7 0 0 0 1.88.34A1.7 1.7 0 0 0 11.7 4.74v-.08h3v.08a1.7 1.7 0 0 0 1.04 1.56 1.7 1.7 0 0 0 1.88-.34l.06-.06 2.12 2.12-.06.06A1.7 1.7 0 0 0 19.4 10a1.7 1.7 0 0 0 1.56 1.04h.08v3h-.08A1.7 1.7 0 0 0 19.4 15Z" />
      </>
    ),
    calendar: (
      <>
        <rect x="3" y="5" width="18" height="16" rx="2" />
        <path d="M16 3v4M8 3v4M3 10h18M8 14h.01M12 14h.01M16 14h.01M8 18h.01M12 18h.01" />
      </>
    ),
    log: (
      <>
        <rect x="4" y="3" width="16" height="18" rx="2" />
        <path d="M8 8h8M8 12h8M8 16h5" />
      </>
    ),
    gps: (
      <>
        <path d="M12 21s7-6.1 7-12a7 7 0 1 0-14 0c0 5.9 7 12 7 12Z" />
        <circle cx="12" cy="9" r="2" />
      </>
    ),
    send: (
      <>
        <path d="m22 2-7 20-4-9-9-4Z" />
        <path d="M22 2 11 13" />
      </>
    ),
    refresh: (
      <>
        <path d="M20 11a8 8 0 1 0 2 5.5" />
        <path d="M20 4v7h-7" />
      </>
    ),
    alert: (
      <>
        <path d="M10.3 3.7 2.4 18a2 2 0 0 0 1.75 3h15.7A2 2 0 0 0 21.6 18L13.7 3.7a2 2 0 0 0-3.4 0Z" />
        <path d="M12 9v4M12 17h.01" />
      </>
    ),
  };
  return (
    <svg
      className="icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name]}
    </svg>
  );
}
function App() {
  const [config, setConfig] = useState<Config>({
    callsign: "SM0ABC",
    grid: "JO89",
    useGPS: false,
    power: 23,
    frequency: 14095600,
  });
  const [draftDirty, setDraftDirty] = useState(false);
  const [status, setStatus] = useState<Status>({
    connected: false,
    simulated: false,
    port: "",
    gpsLocked: false,
    transmitting: false,
    lastTransmission: "",
    nextWindow: "--:--",
    message: "Startar...",
    scheduleEnabled: false,
  });
  const [ports, setPorts] = useState<string[]>([]),
    [port, setPort] = useState(""),
    [logs, setLogs] = useState<string[]>([]),
    [schedule, setSchedule] = useState<Entry[]>([]),
    [scheduleEnabled, setScheduleEnabled] = useState(false),
    [page, setPage] = useState<"overview" | "config" | "schedule" | "log">(
      "overview",
    ),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    const service = api();
    if (!service) return;
    const [
      nextStatus,
      nextConfig,
      nextLogs,
      nextSchedule,
      nextScheduleEnabled,
    ] = await Promise.all([
      service.GetStatus(),
      service.GetConfig(),
      service.GetLogs(),
      service.GetSchedule(),
      service.GetScheduleEnabled(),
    ]);
    setStatus(nextStatus);
    if (!draftDirty) setConfig(nextConfig);
    setLogs(nextLogs);
    setSchedule(nextSchedule);
    setScheduleEnabled(nextScheduleEnabled);
  }, [draftDirty]);
  const refreshPorts = useCallback(async () => {
    try {
      const found = await api()?.ListPorts();
      setPorts(found ?? []);
      if (!port && found?.[0]) setPort(found[0]);
    } catch (error) {
      setNotice(String(error));
    }
  }, [port]);
  useEffect(() => {
    void load();
    void refreshPorts();
    const timer = window.setInterval(() => void load(), 1000);
    return () => window.clearInterval(timer);
  }, [load, refreshPorts]);
  const updateDraft = (update: Config | ((current: Config) => Config)) => {
    setDraftDirty(true);
    setConfig(update);
  };
  const loadFromDevice = async () => {
    try {
      const detected = await api()?.LoadDetectedConfig();
      if (detected) {
        setConfig(detected);
        setDraftDirty(false);
        setNotice(
          "Konfigurationen från beaconens senast mottagna CFG-rad är laddad.",
        );
      }
    } catch (error) {
      setNotice(`Kunde inte läsa in konfigurationen: ${String(error)}`);
    }
  };
  const setMode = async (enabled: boolean) => {
    try {
      await api()?.SetScheduleEnabled(enabled);
      setScheduleEnabled(enabled);
      setNotice(
        enabled
          ? "Schemaläge är aktivt."
          : "Manuellt konfigurationsläge är aktivt. Schemat kör inte.",
      );
    } catch (error) {
      setNotice(`Kunde inte ändra driftläge: ${String(error)}`);
    }
  };
  const connect = async (simulated: boolean) => {
    setBusy(true);
    try {
      const next = await api()?.Connect(port, simulated);
      if (next) setStatus(next);
      setNotice(
        simulated
          ? "Simulerat läge är aktivt."
          : `Ansluten till ${port} vid 9600 baud.`,
      );
    } catch (error) {
      setNotice(`Kunde inte ansluta: ${String(error)}`);
    } finally {
      setBusy(false);
    }
  };
  const apply = async () => {
    setBusy(true);
    try {
      const result = await api()?.ApplyConfig(config);
      setDraftDirty(false);
      setNotice(result ?? "Konfiguration skickad.");
      await load();
    } catch (error) {
      setNotice(`Konfigurationen skickades inte: ${String(error)}`);
    } finally {
      setBusy(false);
    }
  };
  const saveSchedule = async () => {
    try {
      await api()?.SaveSchedule(schedule);
      setNotice("Schemat är sparat lokalt och bevakas av appen.");
    } catch (error) {
      setNotice(`Schemat sparades inte: ${String(error)}`);
    }
  };
  const currentBand = useMemo(
    () => bandFor(config.frequency),
    [config.frequency],
  );
  return (
    <main className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brandmark">
            <Icon name="radio" />
          </span>
          <span>WSPR Beacon</span>
        </div>
        <nav>
          {(
            [
              ["overview", "radio", "Översikt"],
              ["config", "settings", "Konfiguration"],
              ["schedule", "calendar", "Schema"],
              ["log", "log", "Logg"],
            ] as const
          ).map(([id, icon, label]) => (
            <button
              className={page === id ? "nav-item active" : "nav-item"}
              onClick={() => setPage(id)}
              key={id}
            >
              <Icon name={icon} />
              {label}
            </button>
          ))}
        </nav>
        <div className="side-status">
          <span
            className={status.connected ? "status-dot live" : "status-dot"}
          ></span>
          <div>
            <strong>
              {status.connected
                ? status.simulated
                  ? "Simulerad"
                  : "Ansluten"
                : "Frånkopplad"}
            </strong>
            <small>{scheduleEnabled ? "Schemaläge" : "Manuellt läge"}</small>
          </div>
        </div>
        <p className="version">WSPR Beacon · lokal styrning</p>
      </aside>
      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">
              {page === "overview" ? "ÖVERSIKT" : page.toUpperCase()}
            </p>
            <h1>
              {page === "overview"
                ? "Stationens läge"
                : page === "config"
                  ? "Konfiguration"
                  : page === "schedule"
                    ? "Bandplan"
                    : "Seriell logg"}
            </h1>
          </div>
          <div className="connection">
            <span
              className={status.connected ? "status-dot live" : "status-dot"}
            ></span>
            {scheduleEnabled ? "Schemaläge aktivt" : "Manuell konfiguration"}
          </div>
        </header>
        {notice && (
          <div className="notice">
            <Icon name="alert" />
            <span>{notice}</span>
            <button onClick={() => setNotice("")} aria-label="Stäng">
              ×
            </button>
          </div>
        )}
        {page === "overview" && (
          <Overview
            status={status}
            config={config}
            logs={logs}
            schedule={schedule}
            scheduleEnabled={scheduleEnabled}
            onConfig={() => setPage("config")}
            onSchedule={() => setPage("schedule")}
          />
        )}{" "}
        {page === "config" && (
          <Configuration
            config={config}
            setConfig={updateDraft}
            ports={ports}
            port={port}
            setPort={setPort}
            refreshPorts={refreshPorts}
            status={status}
            busy={busy}
            connect={connect}
            apply={apply}
            loadFromDevice={loadFromDevice}
            currentBand={currentBand}
          />
        )}{" "}
        {page === "schedule" && (
          <Schedule
            schedule={schedule}
            setSchedule={setSchedule}
            save={saveSchedule}
            enabled={scheduleEnabled}
            setEnabled={setMode}
          />
        )}{" "}
        {page === "log" && <LogView logs={logs} />}
      </section>
    </main>
  );
}
function Overview({
  status,
  config,
  logs,
  schedule,
  scheduleEnabled,
  onConfig,
  onSchedule,
}: {
  status: Status;
  config: Config;
  logs: string[];
  schedule: Entry[];
  scheduleEnabled: boolean;
  onConfig: () => void;
  onSchedule: () => void;
}) {
  return (
    <div className="overview-grid">
      <section className="frequency-panel">
        <p>AKTUELL FREKVENS</p>
        <div className="frequency">
          {frequencyLabel(config.frequency)} <span>MHz</span>
        </div>
        <div className="frequency-meta">
          <strong>{bandFor(config.frequency)} WSPR</strong>
          <span>•</span>
          <span>
            Nästa sändfönster <b>{status.nextWindow}</b>
          </span>
        </div>
        {status.transmitting && (
          <div className="tx-running">
            <span /> Sändning pågår - konfigurationsändringar är låsta
          </div>
        )}
      </section>
      <section className="station-panel">
        <div>
          <Icon name="radio" />
          <p>ANROPSSIGNAL</p>
          <strong>{config.callsign}</strong>
        </div>
        <div>
          <Icon name="gps" />
          <p>GRID</p>
          <strong>{config.useGPS ? "GPS" : config.grid}</strong>
          <small>{status.gpsLocked ? "GPS-lås" : "Väntar på GPS"}</small>
        </div>
        <div>
          <p>DRIFTLÄGE</p>
          <strong>{scheduleEnabled ? "Schema" : "Manuellt"}</strong>
          <small>
            {scheduleEnabled
              ? "Automatiska bandbyten är på"
              : "Inga automatiska bandbyten"}
          </small>
        </div>
      </section>
      <section className="console-panel">
        <div className="panel-title">
          <span>SENASTE HÄNDELSER</span>
          <button onClick={onConfig}>Ändra konfiguration</button>
        </div>
        <pre>
          {logs.slice(-9).join("\n") || "Anslut en beacon för seriell logg."}
        </pre>
      </section>
      <section className="schedule-panel">
        <div className="panel-title">
          <span>{scheduleEnabled ? "AKTIVT SCHEMA" : "SCHEMA AVSTÄNGT"}</span>
          <button onClick={onSchedule}>Redigera schema</button>
        </div>
        {schedule.length === 0 ? (
          <p className="empty">Inga schemarader ännu.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Start</th>
                <th>Band</th>
                <th>Frekvens</th>
                <th>Dagar</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {schedule
                .filter((e) => e.enabled)
                .map((entry) => (
                  <tr key={entry.id}>
                    <td>{entry.start}</td>
                    <td>{entry.band}</td>
                    <td>{frequencyLabel(entry.frequency)} MHz</td>
                    <td>
                      {entry.days.map((day) => DAY_NAMES[day]).join(", ")}
                    </td>
                    <td>
                      <span className={scheduleEnabled ? "enabled" : ""}>
                        {scheduleEnabled ? "Aktiv" : "Pausad"}
                      </span>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}
function Configuration({
  config,
  setConfig,
  ports,
  port,
  setPort,
  refreshPorts,
  status,
  busy,
  connect,
  apply,
  loadFromDevice,
  currentBand,
}: any) {
  return (
    <div className="configuration-layout">
      <section className="form-panel">
        <div className="form-heading">
          <div>
            <h2>Anslutning</h2>
            <p>Beaconens serieport kör alltid 9600 baud.</p>
          </div>
          <button
            className="icon-button"
            onClick={refreshPorts}
            title="Sök portar"
          >
            <Icon name="refresh" />
          </button>
        </div>
        <div className="port-row">
          <select
            value={port}
            onChange={(e) => setPort(e.target.value)}
            disabled={status.connected && !status.simulated}
          >
            <option value="">Välj serieport...</option>
            {ports.map((item: string) => (
              <option key={item}>{item}</option>
            ))}
          </select>
          {status.connected ? (
            <button
              className="secondary"
              onClick={() =>
                api()
                  ?.Disconnect()
                  .then(() => window.location.reload())
              }
            >
              Koppla från
            </button>
          ) : (
            <>
              <button
                className="secondary"
                disabled={busy || !port}
                onClick={() => connect(false)}
              >
                Anslut
              </button>
              <button
                className="simulated"
                disabled={busy}
                onClick={() => connect(true)}
              >
                Simulera
              </button>
            </>
          )}
        </div>
        <p className="help">
          Simulerat läge provar flödet utan att skicka radiosignal eller öppna
          en USB-port.
        </p>
        <button
          className="secondary load-device"
          disabled={!status.connected}
          onClick={loadFromDevice}
        >
          <Icon name="refresh" /> Ladda mottagen konfiguration
        </button>
        <p className="help">
          Läser beaconens senast mottagna <code>CFG:</code>-rad. Koppla om eller
          starta om beaconen om ingen rad finns.
        </p>
      </section>
      <section className="form-panel">
        <div className="form-heading">
          <div>
            <h2>Sändningsprofil</h2>
            <p>
              Ändringar sparas i beaconens EEPROM när den bekräftar{" "}
              <code>OK</code>.
            </p>
          </div>
        </div>
        <div className="form-grid">
          <label>
            Anropssignal
            <input
              maxLength={6}
              value={config.callsign}
              onChange={(e) =>
                setConfig({ ...config, callsign: e.target.value.toUpperCase() })
              }
            />
          </label>
          <label>
            Maidenhead grid
            <input
              maxLength={4}
              value={config.grid}
              disabled={config.useGPS}
              onChange={(e) =>
                setConfig({ ...config, grid: e.target.value.toUpperCase() })
              }
            />
          </label>
          <label className="toggle-label">
            <input
              type="checkbox"
              checked={config.useGPS}
              onChange={(e) =>
                setConfig({ ...config, useGPS: e.target.checked })
              }
            />
            <span className="switch" /> Hämta grid från GPS
          </label>
          <label>
            Uteffekt (dBm)
            <div className="number-wrap">
              <input
                type="number"
                min="0"
                max="60"
                value={config.power}
                onChange={(e) =>
                  setConfig({ ...config, power: Number(e.target.value) })
                }
              />
              <span>dBm</span>
            </div>
          </label>
        </div>
        <div className="band-picker">
          <p>WSPR-band</p>
          <div>
            {BANDS.map(([band, frequency]) => (
              <button
                key={band}
                className={config.frequency === frequency ? "selected" : ""}
                onClick={() => setConfig({ ...config, frequency })}
              >
                {band}
              </button>
            ))}
          </div>
          <label className="manual-frequency">
            Egen frekvens (Hz)
            <input
              type="number"
              min="1000000"
              max="60000000"
              step="1"
              value={config.frequency}
              onChange={(e) => setConfig({ ...config, frequency: Number(e.target.value) })}
            />
          </label>
          <small>
            Valt: {currentBand} · {frequencyLabel(config.frequency)} MHz
          </small>
        </div>
        <div className="safe-note">
          <Icon name="alert" />
          <span>
            Manuella ändringar skickas direkt. Beaconen tillämpar dem när den
            är redo; schemaläget använder ett eget säkert tidsfönster.
          </span>
        </div>
        <button
          className="primary"
          disabled={!status.connected || busy}
          onClick={apply}
        >
          <Icon name="send" />
          {busy ? "Arbetar..." : "Skicka konfiguration"}
        </button>
      </section>
    </div>
  );
}
function Schedule({
  schedule,
  setSchedule,
  save,
  enabled,
  setEnabled,
}: {
  schedule: Entry[];
  setSchedule: React.Dispatch<React.SetStateAction<Entry[]>>;
  save: () => void;
  enabled: boolean;
  setEnabled: (enabled: boolean) => void;
}) {
  const update = (index: number, field: keyof Entry, value: any) =>
    setSchedule((items) =>
      items.map((item, i) =>
        i === index ? { ...item, [field]: value } : item,
      ),
    );
  const add = () =>
    setSchedule((items) => [
      ...items,
      {
        id: `row-${Date.now()}`,
        days: [1, 2, 3, 4, 5],
        start: "12:00",
        band: "20 m",
        frequency: 14097100,
        enabled: true,
      },
    ]);
  return (
    <section className="schedule-editor">
      <div className="editor-intro">
        <div>
          <h2>Automatiska bandbyten</h2>
          <p>
            {enabled
              ? "Schemat är aktivt och kan byta band mellan sändningarna."
              : "Schemat är avstängt. Du kan redigera raderna utan att beaconen ändras automatiskt."}
          </p>
        </div>
        <button className="primary compact" onClick={save}>
          Spara schema
        </button>
      </div>
      <div className="mode-switch">
        <div>
          <strong>
            {enabled ? "Schemaläge" : "Manuellt konfigurationsläge"}
          </strong>
          <small>
            {enabled
              ? "Automatiska bandbyten är på"
              : "Inga automatiska bandbyten körs"}
          </small>
        </div>
        <label className="active-toggle">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          <span className="switch" /> {enabled ? "På" : "Av"}
        </label>
      </div>
      <div className="schedule-table">
        {schedule.map((entry, index) => (
          <div className="schedule-row" key={entry.id}>
            <label>
              Start
              <input
                type="time"
                value={entry.start}
                onChange={(e) => update(index, "start", e.target.value)}
              />
            </label>
            <label>
              Band
              <select
                value={entry.frequency}
                onChange={(e) => {
                  const f = Number(e.target.value);
                  update(index, "frequency", f);
                  update(index, "band", bandFor(f));
                }}
              >
                {BANDS.map(([band, f]) => (
                  <option value={f} key={band}>
                    {band} · {frequencyLabel(f)} MHz
                  </option>
                ))}
              </select>
            </label>
            <fieldset>
              <legend>Dagar</legend>
              {DAY_NAMES.map((day, d) => (
                <label className="day" key={day}>
                  <input
                    type="checkbox"
                    checked={entry.days.includes(d)}
                    onChange={(e) =>
                      update(
                        index,
                        "days",
                        e.target.checked
                          ? [...entry.days, d]
                          : entry.days.filter((x) => x !== d),
                      )
                    }
                  />
                  {day}
                </label>
              ))}
            </fieldset>
            <label className="active-toggle">
              <input
                type="checkbox"
                checked={entry.enabled}
                onChange={(e) => update(index, "enabled", e.target.checked)}
              />
              <span className="switch" /> Aktiv
            </label>
            <button
              className="remove"
              onClick={() =>
                setSchedule((items) => items.filter((_, i) => i !== index))
              }
            >
              Ta bort
            </button>
          </div>
        ))}
      </div>
      <button className="add-row" onClick={add}>
        + Lägg till bandbyte
      </button>
      <p className="legal">
        <strong>Operatörsansvar:</strong> Kontrollera alltid att valda band,
        effekt och identifiering är tillåtna enligt ditt certifikat och lokala
        regler.
      </p>
    </section>
  );
}
function LogView({ logs }: { logs: string[] }) {
  return (
    <section className="log-view">
      <div className="editor-intro">
        <div>
          <h2>Seriell händelselogg</h2>
          <p>Visar lokala kommandon och svar från beaconen.</p>
        </div>
        <button
          className="secondary"
          onClick={() => navigator.clipboard.writeText(logs.join("\n"))}
        >
          Kopiera logg
        </button>
      </div>
      <pre>{logs.join("\n") || "Inga händelser registrerade."}</pre>
    </section>
  );
}
export default App;
