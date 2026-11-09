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

## 6. Red Hat & Fedora SELinux Configuration
- **Container Volume Contexts (`container_file_t`):** On Fedora, Red Hat Enterprise Linux, CentOS Stream, AlmaLinux, and Rocky Linux, SELinux operates in Enforcing mode by default. Host directories bind-mounted into Docker containers (such as `/data/` and `/opt/sovereign`) require the `container_file_t` context to grant container processes read and write permissions.
- **Automated Installer Handling:** The Sovereign installer automatically detects active SELinux environments, enables permanent `semanage fcontext` policies (or `chcon` runtime labeling), and maintains context integrity across upgrades.
- **Immediate Context Application:** To apply the container label manually to all persistent directories:
  ```bash
  sudo chcon -Rt container_file_t /data /opt/sovereign
  ```
- **Persistent Policy Across Reboots & Relabels:** To ensure the context survives system updates and filesystem relabels, register the rule with `semanage` (`policycoreutils-python-utils`):
  ```bash
  sudo semanage fcontext -a -t container_file_t "/data(/.*)?"
  sudo semanage fcontext -a -t container_file_t "/opt/sovereign(/.*)?"
  sudo restorecon -Rv /data /opt/sovereign
  ```
- **Docker Compose Volume Flags (`:z` vs `:Z`):** When configuring custom compose services that bind-mount directories under `/data`, always append the `:z` flag (shared container volume) rather than `:Z`. Because multiple Sovereign services (Jellyfin, Navidrome, Paperless) share access to `/data`, the `:Z` flag must be avoided as it grants exclusive access to a single container and denies access to all other services.
- **External NAS Mounts (NFS / SMB):** Network filesystems do not support standard SELinux extended attributes (`security.selinux`), causing `chcon` to fail. When mounting external NAS shares under `/data` or `/mnt/nas`:
  - Mount with an explicit SELinux context option in `/etc/fstab` or manual mount commands:
    ```bash
    mount -t nfs -o context="system_u:object_r:container_file_t:s0" nas.local:/volume1/media /mnt/nas
    ```
  - Enable SELinux container network storage booleans if needed:
    ```bash
    sudo setsebool -P virt_use_nfs 1
    sudo setsebool -P virt_use_samba 1
    ```
- **Diagnosing Permission Denials:** If any service reports `Permission denied` when accessing files in `/data`, inspect the SELinux audit log for Access Vector Cache (AVC) rejections:
  ```bash
  sudo ausearch -m avc -ts recent
  ```
  or check the system journal and audit logs:
  ```bash
  sudo journalctl -g AVC
  sudo grep AVC /var/log/audit/audit.log
  ```

