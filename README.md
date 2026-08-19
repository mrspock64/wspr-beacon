# WSPR Beacon

En lokal desktop-app för WSPR Beacon firmware v1.02. Appen är byggd med Go och Wails och körs på macOS, Windows och Linux.

## Användning

1. Anslut beaconens USB-C-port och välj rätt serieport i **Konfiguration**.
2. Klicka **Anslut**. Appen använder alltid 9600 baud enligt firmware-manualen.
3. Ange anropssignal, fast grid eller GPS-grid, effekt och ett av beaconens stödda WSPR-band.
4. Skicka konfigurationen. Appen visar kommandot och beaconens `OK`/`ERR`-svar i loggen.
5. Lägg in tider i **Schema** om bandet ska växla automatiskt.

**Simulera** testar hela användarflödet utan att öppna en serieport eller styra radio.

## Säker schemaläggning

Beaconen sänder på jämna minuter och en sändning tar cirka 110 sekunder. Därför försöker appen endast tillämpa schemat under de sista åtta sekunderna av den efterföljande udda minuten (sekund 52-59). Manuella ändringar blockeras under en beräknad aktiv sändning.

Schema och den senast använda profilen sparas lokalt i användarens konfigurationsmapp. Konfiguration som har bekräftats av beaconen sparas dessutom av enheten i EEPROM.

## Utveckling

Krav: Go 1.23+, Node.js och Wails v2.

```sh
cd frontend
npm install
npm run dev
```

Bygg en macOS-app:

```sh
wails build -platform darwin/arm64
```

För Windows och Linux byggs samma källa med Wails lämpliga målplattform. Serieportsbiblioteket använder systemets USB/TTY-portar och behöver normalt inga drivrutiner för en vanlig USB-serialenhet.

## Kontroll

```sh
go test ./...
```

Operatören ansvarar för att callsign, effekt och valda WSPR-band uppfyller licensvillkor och lokala regler.
