# Guías del electrodoméstico Sovereign (Español)

## 1. Inicio rápido y configuración
- **Instalador de una sola línea:** En cualquier máquina Linux limpia, ejecute `curl -fsSL https://raw.githubusercontent.com/j333c/sovereign/main/install.sh | sudo bash` para instalar Docker automáticamente, desplegar `/opt/sovereign` e iniciar el servidor.
- **Conexiones físicas:** Conecte el cable de alimentación y enchufe el cable de red Ethernet directamente a su rúter. Presione el botón de encendido.
- **Acceso en red local:** Abra `http://sovereign.local` en cualquier dispositivo conectado a su red Wi-Fi. Si su rúter no admite mDNS, consulte la IP asignada en la tabla DHCP de su rúter (ej. `http://192.168.1.50`).
- **Asistente inicial:** Complete los 5 pasos para seleccionar su idioma, verificar la red, inicializar el almacenamiento, activar servicios y definir su contraseña de administrador.

## 2. Almacenamiento y NAS en red
- **Grupo de almacenamiento:** Los discos internos se agrupan en un pool resistente montado en `/data/`. El registro por adelantado (WAL) de SQLite protege la integridad de los datos.
- **Montaje de NAS externo:** En `Configuración > Almacenamiento y grupos` puede enlazar su almacenamiento Synology, QNAP o TrueNAS mediante SMB o NFS. Utilice "Probar conexión" antes de montar.
- **Supervisión de espacio:** La barra inferior muestra el uso en tiempo real. Al superar el 85 % el indicador pasa a ámbar; al 95 % se pausan las descargas para garantizar la estabilidad.

## 3. Medios y zona de entrega
- **Zona de entrega web:** En `/drop` arrastre y suelte películas, series, canciones o audiolibros. Sovereign los procesa e indexa inmediatamente en Jellyfin y Navidrome.
- **Carpeta compartida:** Acceda a `\\sovereign\drop` en Windows o `smb://sovereign.local/drop` en macOS para transferir colecciones completas.
- **Extracción de discos ópticos:** Introduzca un DVD, Blu-ray o CD de audio en una unidad óptica conectada. Se extraerán las pistas sin pérdida, se añadirán carátulas y se expulsará el disco al concluir.

## 4. IA local y privada
- **Aislamiento sin telemetría:** El motor de inferencia local (`llama.cpp`) funciona en una red interna sin salida a Internet. Sus conversaciones y documentos nunca abandonan su hogar.
- **Archivo documental con OCR:** Suba recibos, pólizas o contratos a Paperless-ngx. El sistema extrae automáticamente fechas límite de pago o rescisión para evitar olvidos.
- **Uso compartido de GPU:** La GPU se reserva para respuestas rápidas de IA durante el día y cede su memoria al transcodificador de vídeo 4K tras 5 minutos de inactividad.

## 5. Resiliencia eléctrica y restauración
- **Protección ante cortes eléctricos:** La arquitectura WAL de SQLite resiste apagones abruptos sin corrupción. Los diagnósticos se ejecutan de forma autónoma al reiniciar.
- **Reversión automática en 180 segundos:** Tras cada actualización, Sovereign monitoriza los contenedores durante 180 segundos. Si detecta anomalías, revierte automáticamente a la versión previa.
- **Restauración manual con un clic:** En `Configuración > Servicios` puede regresar a versiones anteriores en cualquier momento. Sus archivos en `/data/` se preservan intactos.

## 6. Configuración de SELinux para Red Hat y Fedora
- **Contextos de volumen para contenedores (`container_file_t`):** En distribuciones como Fedora, Red Hat Enterprise Linux, CentOS Stream, AlmaLinux y Rocky Linux, SELinux se ejecuta de forma predeterminada en modo Enforcing. Los directorios del host montados en contenedores Docker (como `/data/` y `/opt/sovereign`) requieren el contexto `container_file_t` para conceder permisos de lectura y escritura.
- **Configuración automática en el instalador:** El instalador de Sovereign detecta automáticamente si SELinux está activo, registra directivas persistentes mediante `semanage fcontext` (o etiquetas de ejecución con `chcon`) y mantiene la coherencia tras cada actualización.
- **Aplicación inmediata de contexto:** Para aplicar manualmente el etiquetado a todos los directorios persistentes:
  ```bash
  sudo chcon -Rt container_file_t /data /opt/sovereign
  ```
- **Regla persistente ante reinicios y reetiquetados:** Para garantizar que la configuración perdure tras actualizaciones o reetiquetados del sistema, registre la regla con `semanage` (`policycoreutils-python-utils`):
  ```bash
  sudo semanage fcontext -a -t container_file_t "/data(/.*)?"
  sudo semanage fcontext -a -t container_file_t "/opt/sovereign(/.*)?"
  sudo restorecon -Rv /data /opt/sovereign
  ```
- **Banderas de volumen en Docker Compose (`:z` vs. `:Z`):** Al configurar servicios que monten subcarpetas de `/data`, use siempre la bandera `:z` (volumen compartido) en lugar de `:Z`. Dado que múltiples aplicaciones de Sovereign (Jellyfin, Navidrome, Paperless) acceden simultáneamente a `/data`, la opción `:Z` confiere acceso exclusivo a un solo contenedor y deniega el acceso a los demás servicios.
- **Almacenamiento NAS externo (NFS / SMB):** Los sistemas de archivos en red no admiten atributos extendidos estándar de SELinux (`security.selinux`), lo que provoca fallos con `chcon`. Al montar recursos compartidos en `/data` o `/mnt/nas`:
  - Monte con la opción explícita de contexto en `/etc/fstab` o por comando:
    ```bash
    mount -t nfs -o context="system_u:object_r:container_file_t:s0" nas.local:/volume1/media /mnt/nas
    ```
  - Active las variables booleanas de SELinux para almacenamiento en red si es preciso:
    ```bash
    sudo setsebool -P virt_use_nfs 1
    sudo setsebool -P virt_use_samba 1
    ```
- **Diagnóstico de denegaciones de acceso:** Si algún servicio genera un error de `Permission denied` al acceder a `/data`, consulte los bloqueos de AVC en el registro de auditoría de SELinux:
  ```bash
  sudo ausearch -m avc -ts recent
  ```
  o inspeccione el registro del sistema y de auditoría:
  ```bash
  sudo journalctl -g AVC
  sudo grep AVC /var/log/audit/audit.log
  ```

