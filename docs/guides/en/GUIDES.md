# Sovereign Home Appliance Guides (English)

## 1. Quickstart & Setup
- **One-Line Turnkey Installer:** On any clean Linux machine, run `curl -fsSL https://raw.githubusercontent.com/j333c/sovereign/main/install.sh | sudo bash` to automatically provision Docker, deploy `/opt/sovereign`, and start the appliance.
- **Hardware Connections:** Connect your Sovereign server to AC power and plug an Ethernet cable directly into your home router. Press the power button.
- **Local Network Discovery:** Open `http://sovereign.local` on any device connected to your Wi-Fi. If your router blocks mDNS multicast, access the router's DHCP list and enter the IP assigned to `sovereign` (e.g. `http://192.168.1.50`).
- **First-Run Wizard:** Complete the 5-step wizard to select your language, check network connectivity, initialize storage, pick initial services, and set your master administrator password.

## 2. Storage & Network NAS
- **Local Storage Pool:** Internal drives are pooled together under `/data/`. SQLite Write-Ahead Logging (WAL) ensures atomic durability.
- **External NAS Mounts:** Navigate to `Settings > Storage & Pools` to mount your Synology, QNAP, or TrueNAS share over SMB or NFS. Always test credentials using "Test Connection".
- **Capacity Monitoring:** The pinned bottom telemetry bar monitors pool space in real time. Usage above 85% flags an amber notice; 95% pauses automated background downloads.

## 3. Media Ingestion & Dropzone
- **Web Dropzone:** Visit `/drop` to drag and drop movies, television episodes, music, and audiobooks into your library with instant indexing.
- **Network Share:** Access `\\sovereign\drop` on Windows or `smb://sovereign.local/drop` on macOS to transfer large file batches.
- **Optical Disc Ingestion:** Insert movie DVDs, Blu-rays, or music CDs into an attached USB or SATA optical drive. Tracks are ripped losslessly with poster art and ejected upon completion.

## 4. Private Local AI & Document OCR
- **Zero-Telemetry Boundary:** The local `llama.cpp` inference engine runs inside an isolated internal Docker bridge without internet access. Prompts are never transmitted off the appliance.
- **Document Management:** Ingest documents via web upload, mobile scanner, or automated IMAP email polling into Paperless-ngx. Key deadlines and payment obligations are automatically surfaced.
- **Smart GPU Arbitration:** GPU VRAM prioritizes interactive AI during daytime hours and surrenders VRAM to 4K video transcoding after 5 minutes of idle time.

## 5. Power & Rollback Safety
- **Power Cut Protection:** SQLite WAL journaling guarantees database consistency even after an abrupt power outage. System diagnostics run automatically on boot.
- **Automated 180s Rollback:** Every update is monitored for 180 seconds. If any container crashes or fails health verification, the appliance automatically rolls back to the previous snapshot.
- **One-Click Manual Restore:** Administrators can revert any individual service to its previous stable revision at any time in `Settings > Services`. User data under `/data/` is strictly preserved.
