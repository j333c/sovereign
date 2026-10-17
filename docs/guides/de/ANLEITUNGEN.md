# Sovereign-Heimserver Anleitungen (Deutsch)

## 1. Schnellstart & Einrichtung
- **Hardware anschließen:** Verbinden Sie das Stromkabel und schließen Sie den Gigabit-Ethernet-Port direkt an Ihren Internet-Router an. Schalten Sie das Gerät ein.
- **Server im Heimnetzwerk aufrufen:** Öffnen Sie `http://sovereign.local` in einem beliebigen Browser im selben WLAN. Falls Ihr Router kein mDNS unterstützt, suchen Sie in der DHCP-Tabelle Ihres Routers nach `sovereign` und rufen Sie die IP auf (z. B. `http://192.168.1.50`).
- **Einrichtungsassistent:** Der 5-stufige Assistent leitet Sie durch Sprachauswahl, Netzwerkprüfung, Speicherinitialisierung, Dienstauswahl und die Vergabe des Administratorpassworts.

## 2. Speicher & Netzwerk-NAS
- **Lokaler Speicherpool:** Interne Festplatten werden zu einem robusten Speicherpool unter `/data/` zusammengefasst. SQLite mit Write-Ahead-Log (WAL) sichert maximale Datenintegrität.
- **Externes NAS einbinden:** Unter `Einstellungen > Speicher & Pools` binden Sie Synology-, QNAP- oder TrueNAS-Freigaben über SMB oder NFS ein. Nutzen Sie vor dem Einbinden den Button "Verbindung testen".
- **Speicherkapazität überwachen:** Die Statusleiste am unteren Rand zeigt die Belegung in Echtzeit an. Ab 85 % wechselt die Anzeige auf Bernstein; ab 95 % pausieren automatische Downloads.

## 3. Medien & Dropzone
- **Web-Dropzone:** Unter `/drop` ziehen Sie Filme, Serien, Musikalben und Hörbücher per Drag & Drop direkt auf den Server. Dateien werden automatisch in Jellyfin und Navidrome indiziert.
- **Netzwerkfreigabe:** Unter Windows erreichen Sie den Server über `\\sovereign\drop`, auf dem Mac über `smb://sovereign.local/drop`.
- **Automatisches Disc-Rippen:** Legen Sie Film-DVDs, Blu-rays oder Musik-CDs in ein angeschlossenes optisches Laufwerk ein. Titel werden verlustfrei kopiert, mit Covern versehen und nach Fertigstellung ausgeworfen.

## 4. Private lokale KI & Dokumentenablage
- **Keine Telemetrie:** Die lokale Inferenz-Engine (`llama.cpp`) läuft in einem isolierten Docker-Netzwerk ohne Internetverbindung. Prompts und private Daten verlassen das Gerät zu keinem Zeitpunkt.
- **Dokumentenmanagement:** Laden Sie Dokumente per Web, mobiler Scan-App oder automatischer IMAP-E-Mail-Abfrage in Paperless-ngx. Zahlungsfristen und Kündigungstermine werden automatisch erkannt.
- **Intelligente GPU-Aufteilung:** Tagsüber steht die GPU vorrangig der KI zur Verfügung; nach 5 Minuten Inaktivität wird der Grafikspeicher für ruckelfreie 4K-Videowiedergabe freigegeben.

## 5. Ausfallsicherheit & Rollback
- **Schutz bei Stromausfall:** Der SQLite-WAL schützt Datenbanken vor Beschädigung bei abruptem Stromausfall. Nach Neustart führt das System automatische Integritätsprüfungen durch.
- **Automatisches 180s-Rollback:** Updates werden 180 Sekunden lang auf Stabilität überwacht. Bei Fehlern setzt sich das System selbstständig auf den vorherigen Schnappschuss zurück.
- **Manuelles Ein-Klick-Rollback:** In `Einstellungen > Dienste` können Sie jederzeit manuell auf frühere Versionen zurückspringen. Ihre Dateien und Dokumente unter `/data/` bleiben dabei unberührt.
