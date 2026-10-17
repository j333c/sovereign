export type Language = "en" | "de" | "es" | "fr";

export interface DashboardTranslations {
  crestTitle: string;
  crestSubtitle: string;
  allSystemsOperational: string;
  attentionRequired: string;
  systemError: string;
  dropMedia: string;
  settings: string;
  languageLabel: string;
  runningHealthy: string;
  stoppedError: string;
  restartService: string;
  rollbackVersion: string;
  downloadLogs: string;
  optionsFor: string;
  storage: string;
  cpu: string;
  ram: string;
  nas: string;
  nasConnected: string;
  nasDegraded: string;
  nasDisconnected: string;
  nasNotConfigured: string;
  opticalDrive: string;
  opticalIdle: string;
  gpuSharing: string;
  gpuActive: string;
  adminAuthTitle: string;
  adminAuthDesc: string;
  adminPasswordLabel: string;
  adminPasswordPlaceholder: string;
  unlock: string;
  cancel: string;
  invalidPassword: string;
  videoTitle: string;
  videoSubtitle: string;
  videoMetric: string;
  videoDesc: string;
  videoAction: string;
  audioTitle: string;
  audioSubtitle: string;
  audioMetric: string;
  audioDesc: string;
  audioAction: string;
  dmsTitle: string;
  dmsSubtitle: string;
  dmsMetric: string;
  dmsDesc: string;
  dmsDescDisconnected: string;
  dmsMailboxDisconnectedReassurance: string;
  rollbackReassurance: string;
  dmsAction: string;
  aiTitle: string;
  aiSubtitle: string;
  aiMetric: string;
  aiDesc: string;
  aiAction: string;
  aiWarmingUpReassurance: string;
  aiCpuOptimized: string;
  personaHouseholdName: string;
  personaHouseholdDesc: string;
  personaLegalAdvisorName: string;
  personaLegalAdvisorDesc: string;
  personaCreativeName: string;
  personaCreativeDesc: string;
  personaTechSupportName: string;
  personaTechSupportDesc: string;
  ragTruncationNotice: string;
  ragNoDocumentsFound: string;
  ragDbUnprovisionedNotice: string;
  connectingTelemetry: string;
  liveTelemetry: string;
  reconnecting: string;
  healthPopoverHealthy: string;
  healthPopoverAttention: string;
  systemHealthDetails: string;
  starting: string;
  restarting: string;
  pausedMemoryUse: string;
  videoPausedMemoryUse: string;
  restartSuccess: string;
  restartFailed: string;
  rollbackPreservedNote: string;
  currentVersionLabel: string;
  previousVersionLabel: string;
  snapshotTimestampLabel: string;
  rollbackSuccess: string;
  rollbackFailed: string;
  rollingBack: string;
  noPriorRevisions: string;
  walAndConfig: string;
  diagnosticLogsDownloaded: string;
  close: string;
  inactive: string;
  dropzoneTitle: string;
  dropzoneSubtitle: string;
  categoryMovies: string;
  categoryTV: string;
  categoryMusic: string;
  categoryAudiobooks: string;
  dragDropPrompt: string;
  supportedFormats: string;
  networkShareTitle: string;
  networkShareDesc: string;
  copied: string;
  copyPath: string;
  recentUploads: string;
  noRecentUploads: string;
  uploading: string;
  uploadSuccess: string;
  onlyMediaAccepted: string;
  settingsTitle: string;
  backToDashboard: string;
  tabGeneral: string;
  tabServices: string;
  tabAI: string;
  tabSystem: string;
  aiEngineTitle: string;
  aiEngineSubtitle: string;
  flashAttentionPill: string;
  defaultSetupBadge: string;
  cpuOptimizedBadge: string;
  activeModelCardTitle: string;
  ramFootprint: string;
  vramFootprint: string;
  inferenceSpeed: string;
  tokensPerSec: string;
  noActiveModel: string;
  installedModelsTitle: string;
  modelNameCol: string;
  modelSizeCol: string;
  modelModifiedCol: string;
  modelStatusCol: string;
  modelActionsCol: string;
  activateModel: string;
  activatingModel: string;
  activeBadge: string;
  deleteModel: string;
  confirmDeleteModel: string;
  noModelsInstalled: string;
  downloadManagerTitle: string;
  downloadManagerSubtitle: string;
  modelUrlLabel: string;
  modelUrlPlaceholder: string;
  sha256Label: string;
  sha256Placeholder: string;
  customFilenameLabel: string;
  startDownload: string;
  downloadingState: string;
  storageWarning: string;
  cancelDownload: string;
  downloadSpeed: string;
  downloadETA: string;
  generationPresetLabel: string;
  presetPrecise: string;
  presetBalanced: string;
  presetCreative: string;
  contextLengthLabel: string;
  contextLengthDesc: string;
  gpuLayersLabel: string;
  gpuLayersDesc: string;
  cpuOnlyLocked: string;
  idleTimeoutLabel: string;
  idleTimeoutDesc: string;
  reloadEngine: string;
  reloadingEngine: string;
  reloadSuccess: string;
  viewInferenceLogs: string;
  logsModalTitle: string;
  logsModalDesc: string;
  noLogsRecorded: string;
  logTimestamp: string;
  logPromptTokens: string;
  logCompletionTokens: string;
  logLatency: string;
  logSpeed: string;
  logStatus: string;
  logError: string;
  closeLogs: string;

  // Storage & NAS
  tabStorage: string;
  storagePoolStatus: string;
  detectedDisksTitle: string;
  diskDeviceCol: string;
  diskModelCol: string;
  diskCapacityCol: string;
  diskTypeCol: string;
  nasConfigTitle: string;
  nasConfigDesc: string;
  nasHostLabel: string;
  nasShareLabel: string;
  nasUsernameLabel: string;
  nasPasswordLabel: string;
  nasTestConnection: string;
  nasTesting: string;
  nasMountButton: string;
  nasMounting: string;
  nasTestSuccess: string;
  nasMountSuccess: string;
  storageHealthy: string;
  storageDegraded: string;
  totalCapacity: string;
  usedSpace: string;
  freeSpace: string;
  noDisksDetected: string;

  // Destructive actions
  rebootTitle: string;
  rebootDesc: string;
  rebootConfirmButton: string;
  rebootingState: string;
  factoryResetTitle: string;
  factoryResetDesc: string;
  factoryResetPrompt: string;
  factoryResetConfirmButton: string;
  factoryResettingState: string;
  deactivateTitle: string;
  deactivateDesc: string;
  deactivateConfirmButton: string;
  deactivatingState: string;
  typeResetToConfirm: string;
}

export const translations: Record<Language, DashboardTranslations> = {
  en: {
    crestTitle: "Sovereign",
    crestSubtitle: "Appliance Management",
    allSystemsOperational: "All Systems Operational",
    attentionRequired: "Attention Required",
    systemError: "System Error",
    dropMedia: "Drop Media",
    settings: "Settings",
    languageLabel: "Language",
    runningHealthy: "Running Healthy",
    stoppedError: "Stopped / Error",
    restartService: "Restart Service",
    rollbackVersion: "Roll Back to Previous Version",
    downloadLogs: "Download Diagnostic Logs",
    optionsFor: "Options for",
    storage: "Internal Storage",
    cpu: "CPU",
    ram: "RAM",
    nas: "NAS",
    nasConnected: "Connected",
    nasDegraded: "Degraded",
    nasDisconnected: "Disconnected",
    nasNotConfigured: "Not Configured",
    opticalDrive: "Optical Drive",
    opticalIdle: "Idle · Ready for disc",
    gpuSharing: "Smart Hardware Sharing",
    gpuActive: "Daytime (AI Priority)",
    adminAuthTitle: "Administrator Authentication",
    adminAuthDesc: "Enter your administrator password to access system settings and privileged controls.",
    adminPasswordLabel: "Administrator Password",
    adminPasswordPlaceholder: "Enter admin password",
    unlock: "Unlock",
    cancel: "Cancel",
    invalidPassword: "Incorrect administrator password",
    videoTitle: "Video Streaming",
    videoSubtitle: "Jellyfin",
    videoMetric: "124 Movies",
    videoDesc: "4K Transcoding Ready",
    videoAction: "Open Streaming UI",
    audioTitle: "Music & Audiobooks",
    audioSubtitle: "Navidrome / Audiobookshelf",
    audioMetric: "Mobile Phone Sync Ready",
    audioDesc: "Lossless Audio Library",
    audioAction: "Open Audio UI",
    dmsTitle: "Document Archive",
    dmsSubtitle: "Paperless-ngx",
    dmsMetric: "Automatic Text Search Active",
    dmsDesc: "Mailbox Connected",
    dmsDescDisconnected: "Mailbox Disconnected",
    dmsMailboxDisconnectedReassurance: "Mailbox disconnected: We couldn't connect to your email account. Please check your email password in Settings.",
    rollbackReassurance: "Update couldn't start properly. We've safely restored your previous working version.",
    dmsAction: "Open Documents UI",
    aiTitle: "Local AI Assistant",
    aiSubtitle: "Open WebUI / llama.cpp",
    aiMetric: "Private Family Search",
    aiDesc: "100% On-Device",
    aiAction: "Open Chat UI",
    aiWarmingUpReassurance: "Waking up private AI (ready in 5s)...",
    aiCpuOptimized: "CPU-Optimized Mode",
    personaHouseholdName: "Household Assistant",
    personaHouseholdDesc: "Everyday household helper, task organization, meal planning, general questions.",
    personaLegalAdvisorName: "Legal & Contract Advisor",
    personaLegalAdvisorDesc: "Specialized in consumer rights, statutory revocation, warranty obligations, and formal notice templates.",
    personaCreativeName: "Creative Writer",
    personaCreativeDesc: "Brainstorming, storytelling, family celebration speeches, imaginative writing.",
    personaTechSupportName: "Technical Support",
    personaTechSupportDesc: "Practical step-by-step troubleshooting for home networking, electronics, and household appliances.",
    ragTruncationNotice: "Note: Document excerpts were prioritized to fit your active context window.",
    ragNoDocumentsFound: "No matching documents were found in your household archive to answer this question.",
    ragDbUnprovisionedNotice: "Document archive database is unprovisioned or empty.",
    connectingTelemetry: "Connecting to telemetry stream...",
    liveTelemetry: "Live telemetry stream active",
    reconnecting: "Reconnecting to telemetry...",
    healthPopoverHealthy: "All core hardware, database integrity, and container services are running healthy.",
    healthPopoverAttention: "Some subsystems require attention. Check individual service cards for details.",
    systemHealthDetails: "System Health Details",
    starting: "Starting...",
    restarting: "Restarting...",
    pausedMemoryUse: "Paused due to high memory use. Automatically restarting...",
    videoPausedMemoryUse: "Video Streaming paused due to high memory use. Automatically restarting...",
    restartSuccess: "Service restarted successfully",
    restartFailed: "Failed to restart service",
    rollbackPreservedNote: "Your media, files, and documents are preserved and will not be touched.",
    currentVersionLabel: "Current Version",
    previousVersionLabel: "Previous Version",
    snapshotTimestampLabel: "Backup Snapshot",
    rollbackSuccess: "Service rolled back successfully",
    rollbackFailed: "Failed to roll back service",
    rollingBack: "Rolling back...",
    noPriorRevisions: "No prior version available",
    walAndConfig: "SQLite WAL + Config",
    diagnosticLogsDownloaded: "Diagnostic logs downloaded",
    close: "Close",
    inactive: "Inactive",
    dropzoneTitle: "Media Dropzone",
    dropzoneSubtitle: "Add movies, TV shows, music, and audiobooks to your home server",
    categoryMovies: "Movies",
    categoryTV: "TV Shows",
    categoryMusic: "Music",
    categoryAudiobooks: "Audiobooks",
    dragDropPrompt: "Drop files here or click to browse",
    supportedFormats: "Supports MP4, MKV, WebM, AVI, TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC",
    networkShareTitle: "Home Network Folder",
    networkShareDesc: "Drop files here or copy to \\\\sovereign\\drop on your home network.",
    copied: "Copied!",
    copyPath: "Copy path",
    recentUploads: "Recent Uploads",
    noRecentUploads: "No media files uploaded yet.",
    uploading: "Uploading...",
    uploadSuccess: "Media uploaded successfully and added to your library.",
    onlyMediaAccepted: "Only media files are accepted. Please upload video or audio files.",
    settingsTitle: "Appliance Settings",
    backToDashboard: "Back to Dashboard",
    tabGeneral: "General",
    tabServices: "Services & Add-ons",
    tabAI: "AI Backend & Models",
    tabSystem: "System & Hardware",
    aiEngineTitle: "AI Inference Engine (llama.cpp)",
    aiEngineSubtitle: "Administer local GGUF models, tune GPU offload, and inspect inference telemetry.",
    flashAttentionPill: "FlashAttention Active · KV Cache Optimized",
    defaultSetupBadge: "Default Setup Active",
    cpuOptimizedBadge: "CPU-Optimized Mode Active",
    activeModelCardTitle: "Active Model",
    ramFootprint: "RAM Footprint",
    vramFootprint: "VRAM Footprint",
    inferenceSpeed: "Inference Speed",
    tokensPerSec: "tokens/sec",
    noActiveModel: "No Model Loaded",
    installedModelsTitle: "Installed Models",
    modelNameCol: "Model Name",
    modelSizeCol: "Size",
    modelModifiedCol: "Modified",
    modelStatusCol: "Status",
    modelActionsCol: "Actions",
    activateModel: "Activate",
    activatingModel: "Activating...",
    activeBadge: "Active",
    deleteModel: "Delete",
    confirmDeleteModel: "Are you sure you want to delete this model?",
    noModelsInstalled: "No GGUF models installed. Download a model below to enable local AI.",
    downloadManagerTitle: "Download New Model",
    downloadManagerSubtitle: "Resumable background download with SHA-256 verification and storage safety checks.",
    modelUrlLabel: "Model Download URL (.gguf)",
    modelUrlPlaceholder: "https://huggingface.co/.../model.gguf",
    sha256Label: "Optional SHA-256 Checksum",
    sha256Placeholder: "64-character hex checksum (optional)",
    customFilenameLabel: "Custom Filename (optional)",
    startDownload: "Download Model",
    downloadingState: "Downloading...",
    storageWarning: "Insufficient storage: {modelSize} model requires at least {requiredSpace} free space (5 GB system safety reserve).",
    cancelDownload: "Cancel Download",
    downloadSpeed: "Speed",
    downloadETA: "ETA",
    generationPresetLabel: "Generation Intent Preset",
    presetPrecise: "Precise & Factual",
    presetBalanced: "Balanced Assistant",
    presetCreative: "Creative Drafting",
    contextLengthLabel: "Context Window Size",
    contextLengthDesc: "Maximum conversation and document memory length (KV-cache memory: ~{mem}).",
    gpuLayersLabel: "GPU Layer Offload (-ngl)",
    gpuLayersDesc: "Transformer layers offloaded to GPU VRAM.",
    cpuOnlyLocked: "Locked to CPU (No dedicated GPU detected)",
    idleTimeoutLabel: "Dynamic Idle Surrender Timeout",
    idleTimeoutDesc: "Release GPU memory after {minutes} minutes of inactivity (reserves VRAM for video streaming).",
    reloadEngine: "Reload Backend Engine",
    reloadingEngine: "Reloading Engine...",
    reloadSuccess: "AI engine reloaded successfully.",
    viewInferenceLogs: "View Inference Logs",
    logsModalTitle: "Scrubbed AI Inference Logs",
    logsModalDesc: "Recent inference queries. Prompts and personal data are strictly redacted for privacy.",
    noLogsRecorded: "No inference queries recorded yet.",
    logTimestamp: "Timestamp",
    logPromptTokens: "Prompt Tokens",
    logCompletionTokens: "Completion Tokens",
    logLatency: "Latency",
    logSpeed: "Speed",
    logStatus: "Status",
    logError: "Error",
    closeLogs: "Close",
    tabStorage: "Storage & Pools",
    storagePoolStatus: "Storage Pool Status",
    detectedDisksTitle: "Detected Physical Disks",
    diskDeviceCol: "Device",
    diskModelCol: "Model",
    diskCapacityCol: "Capacity",
    diskTypeCol: "Type",
    nasConfigTitle: "External NAS Storage",
    nasConfigDesc: "Mount network attached storage via SMB/CIFS for expanded media libraries.",
    nasHostLabel: "Server Host / IP",
    nasShareLabel: "Share Name",
    nasUsernameLabel: "Username",
    nasPasswordLabel: "Password",
    nasTestConnection: "Test NAS Connection",
    nasTesting: "Testing Connection...",
    nasMountButton: "Mount NAS Share",
    nasMounting: "Mounting Share...",
    nasTestSuccess: "NAS connection verified successfully.",
    nasMountSuccess: "NAS share mounted successfully.",
    storageHealthy: "Storage Healthy",
    storageDegraded: "Storage Degraded",
    totalCapacity: "Total Capacity",
    usedSpace: "Used Space",
    freeSpace: "Free Space",
    noDisksDetected: "No physical disks detected.",
    rebootTitle: "Reboot Appliance",
    rebootDesc: "Your server will restart and be unavailable for approximately 60 seconds. Active media streaming will resume automatically after reboot.",
    rebootConfirmButton: "Reboot Appliance",
    rebootingState: "Rebooting...",
    factoryResetTitle: "Factory Data Reset",
    factoryResetDesc: "This will erase all appliance settings and return the system to first-run setup. Please type RESET below to confirm.",
    factoryResetPrompt: "Type RESET to confirm",
    factoryResetConfirmButton: "Perform Factory Reset",
    factoryResettingState: "Resetting...",
    deactivateTitle: "Deactivate Service",
    deactivateDesc: "Deactivating this service will stop its background processes. Your files, media, and data in /data/ are completely safe and preserved.",
    deactivateConfirmButton: "Deactivate Service",
    deactivatingState: "Deactivating...",
    typeResetToConfirm: "Type RESET to confirm",
  },
  de: {
    crestTitle: "Sovereign",
    crestSubtitle: "Appliance-Verwaltung",
    allSystemsOperational: "Alle Systeme betriebsbereit",
    attentionRequired: "Aufmerksamkeit erforderlich",
    systemError: "Systemfehler",
    dropMedia: "Medien ablegen",
    settings: "Einstellungen",
    languageLabel: "Sprache",
    runningHealthy: "Läuft einwandfrei",
    stoppedError: "Gestoppt / Fehler",
    restartService: "Dienst neu starten",
    rollbackVersion: "Auf vorherige Version zurücksetzen",
    downloadLogs: "Diagnoseprotokolle herunterladen",
    optionsFor: "Optionen für",
    storage: "Interner Speicher",
    cpu: "CPU",
    ram: "RAM",
    nas: "NAS",
    nasConnected: "Verbunden",
    nasDegraded: "Eingeschränkt",
    nasDisconnected: "Getrennt",
    nasNotConfigured: "Nicht konfiguriert",
    opticalDrive: "Optisches Laufwerk",
    opticalIdle: "Bereit für Disc",
    gpuSharing: "Hardware-Aufteilung",
    gpuActive: "Tagmodus (KI-Priorität)",
    adminAuthTitle: "Administrator-Authentifizierung",
    adminAuthDesc: "Geben Sie Ihr Administratorpasswort ein, um auf Systemeinstellungen zuzugreifen.",
    adminPasswordLabel: "Administratorpasswort",
    adminPasswordPlaceholder: "Passwort eingeben",
    unlock: "Entsperren",
    cancel: "Abbrechen",
    invalidPassword: "Falsches Administratorpasswort",
    videoTitle: "Video-Streaming",
    videoSubtitle: "Jellyfin",
    videoMetric: "124 Filme",
    videoDesc: "4K-Transkodierung bereit",
    videoAction: "Streaming-UI öffnen",
    audioTitle: "Musik & Hörbücher",
    audioSubtitle: "Navidrome / Audiobookshelf",
    audioMetric: "Mobil-Synchronisierung bereit",
    audioDesc: "Verlustfreie Audio-Bibliothek",
    audioAction: "Audio-UI öffnen",
    dmsTitle: "Dokumentenarchiv",
    dmsSubtitle: "Paperless-ngx",
    dmsMetric: "Automatische Textsuche aktiv",
    dmsDesc: "Postfach verbunden",
    dmsDescDisconnected: "Postfach getrennt",
    dmsMailboxDisconnectedReassurance: "Postfach getrennt: Verbindung zum E-Mail-Konto fehlgeschlagen. Bitte überprüfen Sie Ihr Passwort in den Einstellungen.",
    rollbackReassurance: "Das Update konnte nicht ordnungsgemäß gestartet werden. Ihre vorherige funktionierende Version wurde sicher wiederhergestellt.",
    dmsAction: "Dokumenten-UI öffnen",
    aiTitle: "Lokaler KI-Assistent",
    aiSubtitle: "Open WebUI / llama.cpp",
    aiMetric: "Private Familiensuche",
    aiDesc: "100% Lokal",
    aiAction: "Chat-UI öffnen",
    aiWarmingUpReassurance: "Privates KI-Modell wird gestartet (bereit in 5s)...",
    aiCpuOptimized: "CPU-optimierter Modus",
    personaHouseholdName: "Haushaltsassistent",
    personaHouseholdDesc: "Alltäglicher Haushaltshelfer für Aufgabenorganisation, Menüplanung und Familienalltag.",
    personaLegalAdvisorName: "Rechts- & Vertragsberater",
    personaLegalAdvisorDesc: "Spezialisiert auf deutsches und EU-Verbraucherrecht (BGB § 312g Widerrufsrecht, Gewährleistung, Kündigungsfristen, Mahnungen).",
    personaCreativeName: "Kreativer Autor",
    personaCreativeDesc: "Brainstorming, Storytelling, Festtagsreden, kreative Geschichten und ideenreiche Texte.",
    personaTechSupportName: "Technischer Support",
    personaTechSupportDesc: "Praktische Schritt-für-Schritt-Fehlerbehebung für Heimnetzwerk, Elektronik und Haushaltsgeräte.",
    ragTruncationNotice: "Hinweis: Dokumentenauszüge wurden priorisiert, um in das aktive Kontextfenster zu passen.",
    ragNoDocumentsFound: "In Ihrem Dokumentenarchiv wurden keine passenden Dokumente gefunden, um diese Frage zu beantworten.",
    ragDbUnprovisionedNotice: "Das Dokumentenarchiv ist noch nicht eingerichtet oder leer.",
    connectingTelemetry: "Verbinde mit Telemetriestrom...",
    liveTelemetry: "Live-Telemetriestrom aktiv",
    reconnecting: "Verbindung wird wiederhergestellt...",
    healthPopoverHealthy: "Alle Kernhardware-, Datenbankintegritäts- und Containerdienste laufen einwandfrei.",
    healthPopoverAttention: "Einige Subsysteme erfordern Aufmerksamkeit. Details finden Sie auf den Dienstkarten.",
    systemHealthDetails: "Systemstatus-Details",
    starting: "Wird gestartet...",
    restarting: "Wird neu gestartet...",
    pausedMemoryUse: "Wegen hoher Speicherauslastung pausiert. Automatischer Neustart...",
    videoPausedMemoryUse: "Video-Streaming wegen hoher Speicherauslastung pausiert. Automatischer Neustart...",
    restartSuccess: "Dienst erfolgreich neu gestartet",
    restartFailed: "Fehler beim Neustart des Dienstes",
    rollbackPreservedNote: "Ihre Medien, Dateien und Dokumente bleiben erhalten und werden nicht verändert.",
    currentVersionLabel: "Aktuelle Version",
    previousVersionLabel: "Vorherige Version",
    snapshotTimestampLabel: "Sicherungszeitpunkt",
    rollbackSuccess: "Dienst erfolgreich zurückgesetzt",
    rollbackFailed: "Fehler beim Zurücksetzen des Dienstes",
    rollingBack: "Wird zurückgesetzt...",
    noPriorRevisions: "Keine vorherige Version verfügbar",
    walAndConfig: "SQLite-WAL + Konfiguration",
    diagnosticLogsDownloaded: "Diagnoseprotokolle heruntergeladen",
    close: "Schließen",
    inactive: "Inaktiv",
    dropzoneTitle: "Medien-Dropzone",
    dropzoneSubtitle: "Filme, Serien, Musik und Hörbücher auf deinen Heimserver übertragen",
    categoryMovies: "Filme",
    categoryTV: "Serien",
    categoryMusic: "Musik",
    categoryAudiobooks: "Hörbücher",
    dragDropPrompt: "Dateien hier ablegen oder zum Auswählen klicken",
    supportedFormats: "Unterstützt MP4, MKV, WebM, AVI, TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC",
    networkShareTitle: "Heimnetzwerk-Ordner",
    networkShareDesc: "Dateien hier ablegen oder im Heimnetzwerk nach \\\\sovereign\\drop kopieren.",
    copied: "Kopiert!",
    copyPath: "Pfad kopieren",
    recentUploads: "Kürzliche Uploads",
    noRecentUploads: "Noch keine Mediendateien hochgeladen.",
    uploading: "Wird hochgeladen...",
    uploadSuccess: "Mediendatei erfolgreich hochgeladen und zur Bibliothek hinzugefügt.",
    onlyMediaAccepted: "Es werden nur Mediendateien akzeptiert. Bitte lade Video- oder Audiodateien hoch.",
    settingsTitle: "Appliance-Einstellungen",
    backToDashboard: "Zurück zum Dashboard",
    tabGeneral: "Allgemein",
    tabServices: "Dienste & Add-ons",
    tabAI: "KI-Backend & Modelle",
    tabSystem: "System & Hardware",
    aiEngineTitle: "KI-Inferenz-Engine (llama.cpp)",
    aiEngineSubtitle: "Lokale GGUF-Modelle verwalten, GPU-Offloading anpassen und Inferenz-Telemetrie prüfen.",
    flashAttentionPill: "FlashAttention aktiv · KV-Cache optimiert",
    defaultSetupBadge: "Standardkonfiguration aktiv",
    cpuOptimizedBadge: "CPU-optimierter Modus aktiv",
    activeModelCardTitle: "Aktives Modell",
    ramFootprint: "RAM-Bedarf",
    vramFootprint: "VRAM-Bedarf",
    inferenceSpeed: "Inferenzgeschwindigkeit",
    tokensPerSec: "Tokens/Sek.",
    noActiveModel: "Kein Modell geladen",
    installedModelsTitle: "Installierte Modelle",
    modelNameCol: "Modellname",
    modelSizeCol: "Größe",
    modelModifiedCol: "Geändert",
    modelStatusCol: "Status",
    modelActionsCol: "Aktionen",
    activateModel: "Aktivieren",
    activatingModel: "Wird aktiviert...",
    activeBadge: "Aktiv",
    deleteModel: "Löschen",
    confirmDeleteModel: "Möchten Sie dieses Modell wirklich löschen?",
    noModelsInstalled: "Keine GGUF-Modelle installiert. Laden Sie unten ein Modell herunter, um lokale KI zu aktivieren.",
    downloadManagerTitle: "Neues Modell herunterladen",
    downloadManagerSubtitle: "Fortsetzbarer Hintergrund-Download mit SHA-256-Prüfung und Speicher-Sicherheitsabfrage.",
    modelUrlLabel: "Modell-Download-URL (.gguf)",
    modelUrlPlaceholder: "https://huggingface.co/.../model.gguf",
    sha256Label: "Optionale SHA-256-Prüfsumme",
    sha256Placeholder: "64-stellige Hex-Prüfsumme (optional)",
    customFilenameLabel: "Benutzerdefinierter Dateiname (optional)",
    startDownload: "Modell herunterladen",
    downloadingState: "Wird heruntergeladen...",
    storageWarning: "Nicht genügend Speicherplatz: {modelSize} Modell erfordert mindestens {requiredSpace} freien Speicher (5 GB Sicherheitsreserve).",
    cancelDownload: "Download abbrechen",
    downloadSpeed: "Geschwindigkeit",
    downloadETA: "Verbleibend",
    generationPresetLabel: "Generierungs-Voreinstellung",
    presetPrecise: "Präzise & Faktisch",
    presetBalanced: "Ausgewogener Assistent",
    presetCreative: "Kreatives Formulieren",
    contextLengthLabel: "Kontextfenster-Größe",
    contextLengthDesc: "Maximale Konversations- und Dokumentenspeicherlänge (KV-Cache-Speicher: ~{mem}).",
    gpuLayersLabel: "GPU-Ebenen-Offload (-ngl)",
    gpuLayersDesc: "In den GPU-VRAM ausgelagerte Transformer-Schichten.",
    cpuOnlyLocked: "Auf CPU beschränkt (Keine dedizierte GPU erkannt)",
    idleTimeoutLabel: "Dynamischer Leerlauf-Timeout",
    idleTimeoutDesc: "Gibt den GPU-Speicher nach {minutes} Minuten Inaktivität frei (reserviert VRAM für Videostreaming).",
    reloadEngine: "Backend-Engine neu laden",
    reloadingEngine: "Engine wird neu geladen...",
    reloadSuccess: "KI-Engine erfolgreich neu geladen.",
    viewInferenceLogs: "Inferenz-Protokolle anzeigen",
    logsModalTitle: "Bereinigte KI-Inferenzprotokolle",
    logsModalDesc: "Kürzliche Inferenzanfragen. Prompts und persönliche Daten werden zum Schutz der Privatsphäre streng geschwärzt.",
    noLogsRecorded: "Noch keine Inferenzabfragen aufgezeichnet.",
    logTimestamp: "Zeitstempel",
    logPromptTokens: "Prompt-Tokens",
    logCompletionTokens: "Antwort-Tokens",
    logLatency: "Latenz",
    logSpeed: "Geschwindigkeit",
    logStatus: "Status",
    logError: "Fehler",
    closeLogs: "Schließen",
    tabStorage: "Speicher & Pools",
    storagePoolStatus: "Speicherpool-Status",
    detectedDisksTitle: "Erkannte Festplatten",
    diskDeviceCol: "Gerät",
    diskModelCol: "Modell",
    diskCapacityCol: "Kapazität",
    diskTypeCol: "Typ",
    nasConfigTitle: "Externer NAS-Speicher",
    nasConfigDesc: "Binden Sie Netzwerkspeicher (SMB/CIFS) für erweiterte Medienbibliotheken ein.",
    nasHostLabel: "Server-Host / IP",
    nasShareLabel: "Freigabename",
    nasUsernameLabel: "Benutzername",
    nasPasswordLabel: "Passwort",
    nasTestConnection: "NAS-Verbindung testen",
    nasTesting: "Verbindung wird geprüft...",
    nasMountButton: "NAS-Freigabe einbinden",
    nasMounting: "Wird eingebunden...",
    nasTestSuccess: "NAS-Verbindung erfolgreich überprüft.",
    nasMountSuccess: "NAS-Freigabe erfolgreich eingebunden.",
    storageHealthy: "Speicher fehlerfrei",
    storageDegraded: "Speicher beeinträchtigt",
    totalCapacity: "Gesamtkapazität",
    usedSpace: "Belegter Speicher",
    freeSpace: "Freier Speicher",
    noDisksDetected: "Keine physischen Laufwerke erkannt.",
    rebootTitle: "Server neu starten",
    rebootDesc: "Ihr Server wird neu gestartet und ist für etwa 60 Sekunden nicht erreichbar. Aktive Medienwiedergaben werden danach automatisch fortgesetzt.",
    rebootConfirmButton: "Server neu starten",
    rebootingState: "Wird neu gestartet...",
    factoryResetTitle: "Auf Werkseinstellungen zurücksetzen",
    factoryResetDesc: "Dadurch werden alle Geräteeinstellungen gelöscht und das System in den Einrichtungszustand zurückversetzt. Bitte tippen Sie zur Bestätigung RESET ein.",
    factoryResetPrompt: "Tippen Sie RESET zur Bestätigung ein",
    factoryResetConfirmButton: "Auf Werkseinstellungen zurücksetzen",
    factoryResettingState: "Wird zurückgesetzt...",
    deactivateTitle: "Dienst deaktivieren",
    deactivateDesc: "Durch das Deaktivieren werden Hintergrundprozesse gestoppt. Ihre Dateien, Medien und Dokumente unter /data/ bleiben vollständig und sicher erhalten.",
    deactivateConfirmButton: "Dienst deaktivieren",
    deactivatingState: "Wird deaktiviert...",
    typeResetToConfirm: "Tippen Sie RESET zur Bestätigung ein",
  },
  es: {
    crestTitle: "Sovereign",
    crestSubtitle: "Gestión de electrodoméstico",
    allSystemsOperational: "Todos los sistemas operativos",
    attentionRequired: "Atención requerida",
    systemError: "Error del sistema",
    dropMedia: "Depositar medios",
    settings: "Configuración",
    languageLabel: "Idioma",
    runningHealthy: "En ejecución saludable",
    stoppedError: "Detenido / Error",
    restartService: "Reiniciar servicio",
    rollbackVersion: "Revertir a versión anterior",
    downloadLogs: "Descargar registros de diagnóstico",
    optionsFor: "Opciones para",
    storage: "Almacenamiento interno",
    cpu: "CPU",
    ram: "RAM",
    nas: "NAS",
    nasConnected: "Conectado",
    nasDegraded: "Degradado",
    nasDisconnected: "Desconectado",
    nasNotConfigured: "No configurado",
    opticalDrive: "Unidad óptica",
    opticalIdle: "Inactiva · Lista para disco",
    gpuSharing: "Uso compartido de hardware",
    gpuActive: "Modo diurno (Prioridad IA)",
    adminAuthTitle: "Autenticación de administrador",
    adminAuthDesc: "Ingrese su contraseña de administrador para acceder a configuraciones y controles.",
    adminPasswordLabel: "Contraseña de administrador",
    adminPasswordPlaceholder: "Introduzca contraseña",
    unlock: "Desbloquear",
    cancel: "Cancelar",
    invalidPassword: "Contraseña de administrador incorrecta",
    videoTitle: "Transmisión de vídeo",
    videoSubtitle: "Jellyfin",
    videoMetric: "124 Películas",
    videoDesc: "Transcodificación 4K lista",
    videoAction: "Abrir interfaz de vídeo",
    audioTitle: "Música y audiolibros",
    audioSubtitle: "Navidrome / Audiobookshelf",
    audioMetric: "Sincronización móvil lista",
    audioDesc: "Biblioteca de audio sin pérdidas",
    audioAction: "Abrir interfaz de audio",
    dmsTitle: "Archivo de documentos",
    dmsSubtitle: "Paperless-ngx",
    dmsMetric: "Búsqueda automática de texto",
    dmsDesc: "Buzón conectado",
    dmsDescDisconnected: "Buzón desconectado",
    dmsMailboxDisconnectedReassurance: "Buzón desconectado: No se pudo conectar a su cuenta de correo. Verifique su contraseña en Configuración.",
    rollbackReassurance: "La actualización no se pudo iniciar correctamente. Hemos restaurado de forma segura su versión de trabajo anterior.",
    dmsAction: "Abrir interfaz de documentos",
    aiTitle: "Asistente de IA local",
    aiSubtitle: "Open WebUI / llama.cpp",
    aiMetric: "Búsqueda privada familiar",
    aiDesc: "100% en el dispositivo",
    aiAction: "Abrir interfaz de chat",
    aiWarmingUpReassurance: "Iniciando IA privada (lista en 5s)...",
    aiCpuOptimized: "Modo optimizado para CPU",
    personaHouseholdName: "Asistente del Hogar",
    personaHouseholdDesc: "Ayudante diario del hogar, organización de tareas, planificación de comidas y consultas familiares.",
    personaLegalAdvisorName: "Asesor Legal y Contractual",
    personaLegalAdvisorDesc: "Especializado en derechos del consumidor (LGDCU, desistimiento legal de 14 días, garantías y reclamaciones).",
    personaCreativeName: "Escritor Creativo",
    personaCreativeDesc: "Lluvia de ideas, narraciones, discursos para celebraciones familiares y redacción creativa.",
    personaTechSupportName: "Soporte Técnico",
    personaTechSupportDesc: "Resolución de problemas paso a paso para redes domésticas, electrónica y electrodomésticos.",
    ragTruncationNotice: "Nota: Los extractos de documentos se priorizaron para ajustarse a su ventana de contexto activa.",
    ragNoDocumentsFound: "No se encontraron documentos coincidentes en su archivo para responder a esta pregunta.",
    ragDbUnprovisionedNotice: "La base de datos del archivo de documentos no está aprovisionada o está vacía.",
    connectingTelemetry: "Conectando al flujo de telemetría...",
    liveTelemetry: "Flujo de telemetría en vivo activo",
    reconnecting: "Reconectando a telemetría...",
    healthPopoverHealthy: "Todo el hardware principal, la integridad de la base de datos y los contenedores funcionan correctamente.",
    healthPopoverAttention: "Algunos subsistemas requieren atención. Consulte las tarjetas de servicio individuales.",
    systemHealthDetails: "Detalles del estado del sistema",
    starting: "Iniciando...",
    restarting: "Reiniciando...",
    pausedMemoryUse: "Pausado debido al alto uso de memoria. Reiniciando automáticamente...",
    videoPausedMemoryUse: "Transmisión de vídeo pausada debido al alto uso de memoria. Reiniciando automáticamente...",
    restartSuccess: "Servicio reiniciado con éxito",
    restartFailed: "Error al reiniciar el servicio",
    rollbackPreservedNote: "Sus medios, archivos y documentos se conservan y no se tocarán.",
    currentVersionLabel: "Versión actual",
    previousVersionLabel: "Versión anterior",
    snapshotTimestampLabel: "Copia de seguridad",
    rollbackSuccess: "Servicio revertido con éxito",
    rollbackFailed: "Error al revertir el servicio",
    rollingBack: "Revirtiendo...",
    noPriorRevisions: "No hay versión anterior disponible",
    walAndConfig: "SQLite WAL + Configuración",
    diagnosticLogsDownloaded: "Registros de diagnóstico descargados",
    close: "Cerrar",
    inactive: "Inactivo",
    dropzoneTitle: "Zona de Carga de Medios",
    dropzoneSubtitle: "Añade películas, series, música y audiolibros a tu servidor",
    categoryMovies: "Películas",
    categoryTV: "Series",
    categoryMusic: "Música",
    categoryAudiobooks: "Audiolibros",
    dragDropPrompt: "Arrastra archivos aquí o haz clic para explorar",
    supportedFormats: "Compatible con MP4, MKV, WebM, AVI, TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC",
    networkShareTitle: "Carpeta de Red Local",
    networkShareDesc: "Arrastra archivos aquí o cópialos a \\\\sovereign\\drop en tu red local.",
    copied: "¡Copiado!",
    copyPath: "Copiar ruta",
    recentUploads: "Cargas Recientes",
    noRecentUploads: "Aún no se han subido archivos multimedia.",
    uploading: "Subiendo...",
    uploadSuccess: "Archivo subido correctamente y añadido a tu biblioteca.",
    onlyMediaAccepted: "Solo se aceptan archivos multimedia. Sube archivos de vídeo o audio.",
    settingsTitle: "Configuración del electrodoméstico",
    backToDashboard: "Volver al panel",
    tabGeneral: "General",
    tabServices: "Servicios y complementos",
    tabAI: "Backend de IA y modelos",
    tabSystem: "Sistema y hardware",
    aiEngineTitle: "Motor de inferencia de IA (llama.cpp)",
    aiEngineSubtitle: "Administre modelos GGUF locales, ajuste la descarga de GPU e inspeccione la telemetría de inferencia.",
    flashAttentionPill: "FlashAttention activo · KV Cache optimizado",
    defaultSetupBadge: "Configuración predeterminada activa",
    cpuOptimizedBadge: "Modo optimizado para CPU activo",
    activeModelCardTitle: "Modelo activo",
    ramFootprint: "Huella de RAM",
    vramFootprint: "Huella de VRAM",
    inferenceSpeed: "Velocidad de inferencia",
    tokensPerSec: "tokens/s",
    noActiveModel: "Ningún modelo cargado",
    installedModelsTitle: "Modelos instalados",
    modelNameCol: "Nombre del modelo",
    modelSizeCol: "Tamaño",
    modelModifiedCol: "Modificado",
    modelStatusCol: "Estado",
    modelActionsCol: "Acciones",
    activateModel: "Activar",
    activatingModel: "Activando...",
    activeBadge: "Activo",
    deleteModel: "Eliminar",
    confirmDeleteModel: "¿Seguro que desea eliminar este modelo?",
    noModelsInstalled: "No hay modelos GGUF instalados. Descargue un modelo a continuación para habilitar la IA local.",
    downloadManagerTitle: "Descargar nuevo modelo",
    downloadManagerSubtitle: "Descarga en segundo plano reanudable con verificación SHA-256 y comprobación de seguridad de almacenamiento.",
    modelUrlLabel: "URL de descarga del modelo (.gguf)",
    modelUrlPlaceholder: "https://huggingface.co/.../model.gguf",
    sha256Label: "Suma de comprobación SHA-256 opcional",
    sha256Placeholder: "Suma hexadecimal de 64 caracteres (opcional)",
    customFilenameLabel: "Nombre de archivo personalizado (opcional)",
    startDownload: "Descargar modelo",
    downloadingState: "Descargando...",
    storageWarning: "Espacio insuficiente: el modelo de {modelSize} requiere al menos {requiredSpace} de espacio libre (reserva de seguridad de 5 GB).",
    cancelDownload: "Cancelar descarga",
    downloadSpeed: "Velocidad",
    downloadETA: "Tiempo restante",
    generationPresetLabel: "Preajuste de intención de generación",
    presetPrecise: "Preciso y factual",
    presetBalanced: "Asistente equilibrado",
    presetCreative: "Redacción creativa",
    contextLengthLabel: "Tamaño de la ventana de contexto",
    contextLengthDesc: "Longitud máxima de memoria de conversación y documentos (memoria de caché KV: ~{mem}).",
    gpuLayersLabel: "Descarga de capas en GPU (-ngl)",
    gpuLayersDesc: "Capas del transformador descargadas a la memoria VRAM de la GPU.",
    cpuOnlyLocked: "Bloqueado en CPU (No se detectó GPU dedicada)",
    idleTimeoutLabel: "Tiempo de espera de inactividad dinámico",
    idleTimeoutDesc: "Libera la memoria de la GPU tras {minutes} minutos de inactividad (reserva VRAM para streaming de vídeo).",
    reloadEngine: "Recargar motor de backend",
    reloadingEngine: "Recargando motor...",
    reloadSuccess: "Motor de IA recargado con éxito.",
    viewInferenceLogs: "Ver registros de inferencia",
    logsModalTitle: "Registros de inferencia de IA depurados",
    logsModalDesc: "Consultas de inferencia recientes. Las instrucciones y datos personales están estrictamente censurados.",
    noLogsRecorded: "Aún no se han registrado consultas de inferencia.",
    logTimestamp: "Marca de tiempo",
    logPromptTokens: "Tokens de entrada",
    logCompletionTokens: "Tokens de finalización",
    logLatency: "Latencia",
    logSpeed: "Velocidad",
    logStatus: "Estado",
    logError: "Error",
    closeLogs: "Cerrar",
    tabStorage: "Almacenamiento y Pools",
    storagePoolStatus: "Estado del pool de almacenamiento",
    detectedDisksTitle: "Discos físicos detectados",
    diskDeviceCol: "Dispositivo",
    diskModelCol: "Modelo",
    diskCapacityCol: "Capacidad",
    diskTypeCol: "Tipo",
    nasConfigTitle: "Almacenamiento NAS externo",
    nasConfigDesc: "Monte almacenamiento en red mediante SMB/CIFS para ampliar las bibliotecas de medios.",
    nasHostLabel: "Host / IP del servidor",
    nasShareLabel: "Nombre del recurso",
    nasUsernameLabel: "Usuario",
    nasPasswordLabel: "Contraseña",
    nasTestConnection: "Probar conexión NAS",
    nasTesting: "Probando conexión...",
    nasMountButton: "Montar recurso NAS",
    nasMounting: "Montando...",
    nasTestSuccess: "Conexión NAS verificada con éxito.",
    nasMountSuccess: "Recurso NAS montado con éxito.",
    storageHealthy: "Almacenamiento en buen estado",
    storageDegraded: "Almacenamiento degradado",
    totalCapacity: "Capacidad total",
    usedSpace: "Espacio utilizado",
    freeSpace: "Espacio libre",
    noDisksDetected: "No se detectaron discos físicos.",
    rebootTitle: "Reiniciar electrodoméstico",
    rebootDesc: "Su servidor se reiniciará y no estará disponible durante unos 60 segundos. La reproducción multimedia activa se reanudará automáticamente tras el reinicio.",
    rebootConfirmButton: "Reiniciar electrodoméstico",
    rebootingState: "Reiniciando...",
    factoryResetTitle: "Restablecimiento de fábrica",
    factoryResetDesc: "Esto borrará todas las configuraciones del electrodoméstico y devolverá el sistema a la configuración inicial. Escriba RESET a continuación para confirmar.",
    factoryResetPrompt: "Escriba RESET para confirmar",
    factoryResetConfirmButton: "Restablecer a valores de fábrica",
    factoryResettingState: "Restableciendo...",
    deactivateTitle: "Desactivar servicio",
    deactivateDesc: "Desactivar este servicio detendrá sus procesos en segundo plano. Sus archivos, medios y datos en /data/ están completamente seguros y conservados.",
    deactivateConfirmButton: "Desactivar servicio",
    deactivatingState: "Desactivando...",
    typeResetToConfirm: "Escriba RESET para confirmar",
  },
  fr: {
    crestTitle: "Sovereign",
    crestSubtitle: "Gestion de l'appareil",
    allSystemsOperational: "Tous les systèmes opérationnels",
    attentionRequired: "Attention requise",
    systemError: "Erreur système",
    dropMedia: "Déposer des médias",
    settings: "Paramètres",
    languageLabel: "Langue",
    runningHealthy: "Fonctionnement normal",
    stoppedError: "Arrêté / Erreur",
    restartService: "Redémarrer le service",
    rollbackVersion: "Revenir à la version précédente",
    downloadLogs: "Télécharger les journaux de diagnostic",
    optionsFor: "Options pour",
    storage: "Stockage interne",
    cpu: "CPU",
    ram: "RAM",
    nas: "NAS",
    nasConnected: "Connecté",
    nasDegraded: "Dégradé",
    nasDisconnected: "Déconnecté",
    nasNotConfigured: "Non configuré",
    opticalDrive: "Lecteur optique",
    opticalIdle: "Inactif · Prêt pour disque",
    gpuSharing: "Partage intelligent du matériel",
    gpuActive: "Mode jour (Priorité IA)",
    adminAuthTitle: "Authentification administrateur",
    adminAuthDesc: "Entrez votre mot de passe administrateur pour accéder aux paramètres du système.",
    adminPasswordLabel: "Mot de passe administrateur",
    adminPasswordPlaceholder: "Entrez le mot de passe",
    unlock: "Déverrouiller",
    cancel: "Annuler",
    invalidPassword: "Mot de passe administrateur incorrect",
    videoTitle: "Streaming vidéo",
    videoSubtitle: "Jellyfin",
    videoMetric: "124 Films",
    videoDesc: "Transcodage 4K prêt",
    videoAction: "Ouvrir l'interface vidéo",
    audioTitle: "Musique et livres audio",
    audioSubtitle: "Navidrome / Audiobookshelf",
    audioMetric: "Prêt pour synchronisation mobile",
    audioDesc: "Bibliothèque audio sans perte",
    audioAction: "Ouvrir l'interface audio",
    dmsTitle: "Archives de documents",
    dmsSubtitle: "Paperless-ngx",
    dmsMetric: "Recherche de texte automatique",
    dmsDesc: "Boîte mail connectée",
    dmsDescDisconnected: "Boîte mail déconnectée",
    dmsMailboxDisconnectedReassurance: "Boîte mail déconnectée : Impossible de se connecter à votre compte e-mail. Veuillez vérifier votre mot de passe dans les Paramètres.",
    rollbackReassurance: "La mise à jour n'a pas pu démarrer correctement. Nous avons restauré en toute sécurité votre version précédente fonctionnelle.",
    dmsAction: "Ouvrir l'interface documents",
    aiTitle: "Assistant IA local",
    aiSubtitle: "Open WebUI / llama.cpp",
    aiMetric: "Recherche familiale privée",
    aiDesc: "100% sur l'appareil",
    aiAction: "Ouvrir l'interface de chat",
    aiWarmingUpReassurance: "Réveil de l'IA privée (prêt dans 5s)...",
    aiCpuOptimized: "Mode optimisé pour CPU",
    personaHouseholdName: "Assistant Domestique",
    personaHouseholdDesc: "Aide quotidienne pour la maison, organisation des tâches, planification des repas et vie de famille.",
    personaLegalAdvisorName: "Conseiller Juridique et Contrats",
    personaLegalAdvisorDesc: "Spécialisé dans le droit de la consommation (droit de rétractation de 14 jours, garantie de conformité et résiliations).",
    personaCreativeName: "Auteur Créatif",
    personaCreativeDesc: "Remue-méninges, narration, discours de fête de famille et écriture imaginative.",
    personaTechSupportName: "Support Technique",
    personaTechSupportDesc: "Dépannage pas à pas pour les réseaux domestiques, appareils connectés et équipements du foyer.",
    ragTruncationNotice: "Remarque : Les extraits de documents ont été hiérarchisés pour s'adapter à votre fenêtre de contexte active.",
    ragNoDocumentsFound: "Aucun document correspondant n'a été trouvé dans vos archives pour répondre à cette question.",
    ragDbUnprovisionedNotice: "La base de données d'archives de documents n'est pas configurée ou est vide.",
    connectingTelemetry: "Connexion au flux de télémétrie...",
    liveTelemetry: "Flux de télémétrie en direct actif",
    reconnecting: "Reconnexion à la télémétrie...",
    healthPopoverHealthy: "Tous les composants matériels essentiels, l'intégrité de la base de données et les conteneurs fonctionnent correctement.",
    healthPopoverAttention: "Certains sous-systèmes nécessitent une attention. Consultez les cartes de service individuelles.",
    systemHealthDetails: "Détails de l'état du système",
    starting: "Démarrage en cours...",
    restarting: "Redémarrage en cours...",
    pausedMemoryUse: "Interrompu en raison d'une utilisation élevée de la mémoire. Redémarrage automatique...",
    videoPausedMemoryUse: "Streaming vidéo interrompu en raison d'une utilisation élevée de la mémoire. Redémarrage automatique...",
    restartSuccess: "Service redémarré avec succès",
    restartFailed: "Échec du redémarrage du service",
    rollbackPreservedNote: "Vos médias, fichiers et documents sont préservés et ne seront pas modifiés.",
    currentVersionLabel: "Version actuelle",
    previousVersionLabel: "Version précédente",
    snapshotTimestampLabel: "Instantané de sauvegarde",
    rollbackSuccess: "Service restauré avec succès",
    rollbackFailed: "Échec de la restauration du service",
    rollingBack: "Restauration en cours...",
    noPriorRevisions: "Aucune version précédente disponible",
    walAndConfig: "SQLite WAL + Configuration",
    diagnosticLogsDownloaded: "Journaux de diagnostic téléchargés",
    close: "Fermer",
    inactive: "Inactif",
    dropzoneTitle: "Zone de Dépôt Média",
    dropzoneSubtitle: "Ajoutez des films, séries, musiques et livres audio à votre serveur",
    categoryMovies: "Films",
    categoryTV: "Séries",
    categoryMusic: "Musique",
    categoryAudiobooks: "Livres audio",
    dragDropPrompt: "Déposez des fichiers ici ou cliquez pour parcourir",
    supportedFormats: "Prend en charge MP4, MKV, WebM, AVI, TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC",
    networkShareTitle: "Dossier Réseau Local",
    networkShareDesc: "Déposez vos fichiers ici ou copiez-les vers \\\\sovereign\\drop sur votre réseau.",
    copied: "Copié !",
    copyPath: "Copier le chemin",
    recentUploads: "Téléversements Récents",
    noRecentUploads: "Aucun fichier multimédia téléversé pour le moment.",
    uploading: "Téléversement...",
    uploadSuccess: "Fichier téléversé avec succès et ajouté à votre bibliothèque.",
    onlyMediaAccepted: "Seuls les fichiers multimédias sont acceptés. Veuillez envoyer des fichiers vidéo ou audio.",
    settingsTitle: "Paramètres de l'appareil",
    backToDashboard: "Retour au tableau de bord",
    tabGeneral: "Général",
    tabServices: "Services et extensions",
    tabAI: "Moteur IA et modèles",
    tabSystem: "Système et matériel",
    aiEngineTitle: "Moteur d'inférence IA (llama.cpp)",
    aiEngineSubtitle: "Gérer les modèles GGUF locaux, ajuster le déchargement GPU et inspecter la télémétrie d'inférence.",
    flashAttentionPill: "FlashAttention actif · Cache KV optimisé",
    defaultSetupBadge: "Configuration par défaut active",
    cpuOptimizedBadge: "Mode optimisé CPU actif",
    activeModelCardTitle: "Modèle actif",
    ramFootprint: "Empreinte RAM",
    vramFootprint: "Empreinte VRAM",
    inferenceSpeed: "Vitesse d'inférence",
    tokensPerSec: "tokens/s",
    noActiveModel: "Aucun modèle chargé",
    installedModelsTitle: "Modèles installés",
    modelNameCol: "Nom du modèle",
    modelSizeCol: "Taille",
    modelModifiedCol: "Modifié",
    modelStatusCol: "État",
    modelActionsCol: "Actions",
    activateModel: "Activer",
    activatingModel: "Activation...",
    activeBadge: "Actif",
    deleteModel: "Supprimer",
    confirmDeleteModel: "Êtes-vous sûr de vouloir supprimer ce modèle ?",
    noModelsInstalled: "Aucun modèle GGUF installé. Téléchargez un modèle ci-dessous pour activer l'IA locale.",
    downloadManagerTitle: "Télécharger un nouveau modèle",
    downloadManagerSubtitle: "Téléchargement en arrière-plan reprenable avec vérification SHA-256 et contrôle de sécurité du stockage.",
    modelUrlLabel: "URL de téléchargement du modèle (.gguf)",
    modelUrlPlaceholder: "https://huggingface.co/.../model.gguf",
    sha256Label: "Somme de contrôle SHA-256 facultative",
    sha256Placeholder: "Somme hexadécimale de 64 caractères (facultatif)",
    customFilenameLabel: "Nom de fichier personnalisé (facultatif)",
    startDownload: "Télécharger le modèle",
    downloadingState: "Téléchargement...",
    storageWarning: "Espace insuffisant : le modèle de {modelSize} nécessite au moins {requiredSpace} d'espace libre (réserve de sécurité de 5 Go).",
    cancelDownload: "Annuler le téléchargement",
    downloadSpeed: "Vitesse",
    downloadETA: "Temps restant",
    generationPresetLabel: "Préréglage d'intention de génération",
    presetPrecise: "Précis et factuel",
    presetBalanced: "Assistant équilibré",
    presetCreative: "Rédaction créative",
    contextLengthLabel: "Taille de la fenêtre de contexte",
    contextLengthDesc: "Longueur maximale de la mémoire de conversation et de documents (mémoire cache KV : ~{mem}).",
    gpuLayersLabel: "Déchargement des couches sur GPU (-ngl)",
    gpuLayersDesc: "Couches de transformateur déchargées dans la VRAM du GPU.",
    cpuOnlyLocked: "Verrouillé sur CPU (Aucun GPU dédié détecté)",
    idleTimeoutLabel: "Délai d'inactivité dynamique",
    idleTimeoutDesc: "Libère la mémoire du GPU après {minutes} minutes d'inactivité (réserve la VRAM pour le streaming vidéo).",
    reloadEngine: "Recharger le moteur backend",
    reloadingEngine: "Rechargement du moteur...",
    reloadSuccess: "Moteur IA rechargé avec succès.",
    viewInferenceLogs: "Afficher les journaux d'inférence",
    logsModalTitle: "Journaux d'inférence IA épurés",
    logsModalDesc: "Requêtes d'inférence récentes. Les invites et données personnelles sont strictement masquées.",
    noLogsRecorded: "Aucune requête d'inférence enregistrée pour le moment.",
    logTimestamp: "Horodatage",
    logPromptTokens: "Tokens d'invite",
    logCompletionTokens: "Tokens de complétion",
    logLatency: "Latence",
    logSpeed: "Vitesse",
    logStatus: "État",
    logError: "Erreur",
    closeLogs: "Fermer",
    tabStorage: "Stockage & Pools",
    storagePoolStatus: "État du pool de stockage",
    detectedDisksTitle: "Disques physiques détectés",
    diskDeviceCol: "Périphérique",
    diskModelCol: "Modèle",
    diskCapacityCol: "Capacité",
    diskTypeCol: "Type",
    nasConfigTitle: "Stockage NAS externe",
    nasConfigDesc: "Montez un stockage réseau via SMB/CIFS pour étendre vos bibliothèques multimédias.",
    nasHostLabel: "Hôte / IP du serveur",
    nasShareLabel: "Nom du partage",
    nasUsernameLabel: "Nom d'utilisateur",
    nasPasswordLabel: "Mot de passe",
    nasTestConnection: "Tester la connexion NAS",
    nasTesting: "Test de connexion...",
    nasMountButton: "Monter le partage NAS",
    nasMounting: "Montage en cours...",
    nasTestSuccess: "Connexion NAS vérifiée avec succès.",
    nasMountSuccess: "Partage NAS monté avec succès.",
    storageHealthy: "Stockage sain",
    storageDegraded: "Stockage dégradé",
    totalCapacity: "Capacité totale",
    usedSpace: "Espace utilisé",
    freeSpace: "Espace libre",
    noDisksDetected: "Aucun disque physique détecté.",
    rebootTitle: "Redémarrer l'appareil",
    rebootDesc: "Votre serveur va redémarrer et sera indisponible pendant environ 60 secondes. La lecture multimédia active reprendra automatiquement après le redémarrage.",
    rebootConfirmButton: "Redémarrer l'appareil",
    rebootingState: "Redémarrage en cours...",
    factoryResetTitle: "Restauration d'usine",
    factoryResetDesc: "Cette action effacera tous les paramètres de l'appareil et réinitialisera le système à la configuration initiale. Veuillez taper RESET ci-dessous pour confirmer.",
    factoryResetPrompt: "Tapez RESET pour confirmer",
    factoryResetConfirmButton: "Effectuer la restauration d'usine",
    factoryResettingState: "Réinitialisation...",
    deactivateTitle: "Désactiver le service",
    deactivateDesc: "La désactivation de ce service arrêtera ses processus d'arrière-plan. Vos fichiers, médias et données dans /data/ sont totalement sécurisés et préservés.",
    deactivateConfirmButton: "Désactiver le service",
    deactivatingState: "Désactivation...",
    typeResetToConfirm: "Tapez RESET pour confirmer",
  },
};
