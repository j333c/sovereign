# Sovereign-Heimserver Anleitungen (Deutsch)

## 1. Schnellstart & Einrichtung
- **Ein-Zeilen-Installer:** Führen Sie auf jedem Linux-System `curl -fsSL https://raw.githubusercontent.com/j333c/sovereign/main/install.sh | sudo bash` aus, um Docker automatisch einzurichten, `/opt/sovereign` bereitzustellen und den Server zu starten.
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

## 6. SELinux-Konfiguration für Red Hat & Fedora
- **Container-Volume-Kontexte (`container_file_t`):** Unter Fedora, Red Hat Enterprise Linux, CentOS Stream, AlmaLinux und Rocky Linux läuft SELinux standardmäßig im Enforcing-Modus. Host-Verzeichnisse, die per Bind-Mount in Docker-Container eingebunden werden (wie `/data/` und `/opt/sovereign`), benötigen den SELinux-Kontext `container_file_t` für Lese- und Schreibzugriff.
- **Automatische Erkennung im Installer:** Das Sovereign-Installationsskript erkennt aktive SELinux-Systeme automatisch, registriert dauerhafte `semanage fcontext`-Richtlinien (oder `chcon`-Laufzeitlabels) und bewahrt Kontextkonsistenz bei Upgrades.
- **Sofortige Kontextzuweisung:** Um den Container-Kontext manuell auf alle Datenverzeichnisse anzuwenden:
  ```bash
  sudo chcon -Rt container_file_t /data /opt/sovereign
  ```
- **Dauerhafte Konfiguration über Reboots und Relabels hinweg:** Um sicherzustellen, dass die Richtlinie auch nach System-Updates oder automatischen Dateisystem-Relabels erhalten bleibt, hinterlegen Sie die Regel mit `semanage` (`policycoreutils-python-utils`):
  ```bash
  sudo semanage fcontext -a -t container_file_t "/data(/.*)?"
  sudo semanage fcontext -a -t container_file_t "/opt/sovereign(/.*)?"
  sudo restorecon -Rv /data /opt/sovereign
  ```
- **Docker-Compose-Volume-Flags (`:z` vs. `:Z`):** Bei benutzerdefinierten Diensten, die Unterverzeichnisse von `/data` einbinden, verwenden Sie stets das `:z`-Flag (geteiltes Volume) statt `:Z`. Da mehrere Sovereign-Dienste (Jellyfin, Navidrome, Paperless) gleichzeitig auf `/data` zugreifen, würde `:Z` den Zugriff exklusiv an einen einzelnen Container binden und alle anderen Dienste blockieren.
- **Externes NAS einbinden (NFS / SMB):** Netzwerk-Dateisysteme unterstützen keine regulären SELinux-Attribute (`security.selinux`), weshalb `chcon` fehlschlägt. Beim Einbinden von NAS-Freigaben unter `/data` oder `/mnt/nas`:
  - Binden Sie die Freigabe mit expliziter SELinux-Kontextoption in `/etc/fstab` oder per Mount-Befehl ein:
    ```bash
    mount -t nfs -o context="system_u:object_r:container_file_t:s0" nas.local:/volume1/media /mnt/nas
    ```
  - Aktivieren Sie bei Bedarf die SELinux-Booleans für Netzwerkfreigaben:
    ```bash
    sudo setsebool -P virt_use_nfs 1
    sudo setsebool -P virt_use_samba 1
    ```
- **Fehlerdiagnose bei Zugriffsverweigerung:** Sollte ein Dienst beim Zugriff auf `/data` einen Fehler `Permission denied` melden, prüfen Sie das SELinux-Audit-Protokoll auf AVC-Verweigerungen:
  ```bash
  sudo ausearch -m avc -ts recent
  ```
  oder über das System-Journal und Audit-Log:
  ```bash
  sudo journalctl -g AVC
  sudo grep AVC /var/log/audit/audit.log
  ```

