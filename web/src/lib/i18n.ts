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

  // Bundled Guides & Help
  guidesTitle: string;
  guidesSubtitle: string;
  guideQuickstart: string;
  guideStorage: string;
  guideMedia: string;
  guideAI: string;
  guideResilience: string;

  // Boot screen
  bootSafetyChecked: string;
  bootSafetyVerifying: string;
  bootSafetySuccess: string;
  bootSafetySuccessDesc: string;
  bootSafetyError: string;
  bootSafetyErrorDesc: string;
  bootStorageDegraded: string;
  bootBadgeChecking: string;
  bootBadgeSuccess: string;
  bootBadgeAttention: string;

  // Dropzone & Settings additional
  storageIngestionReady: string;
  destinationLibrary: string;
  mediaCategoryTabs: string;
  daytimeSchedule: string;
  nighttimeSchedule: string;
  interactiveAILabel: string;
  batchTranscodeLabel: string;
  balanceHardwareSharing: string;
  systemMaintenanceDesc: string;
  dailyUpdateWindow: string;
  rollbackProtectionLabel: string;
  rollbackProtectionActive: string;
  applianceHostname: string;
  destructiveControlsTitle: string;
  destructiveControlsDesc: string;

  // Hardware Tiers & Calculator (Story 7.1)
  hardwareTitle: string;
  hardwareSubtitle: string;
  tierLightTitle: string;
  tierLightDesc: string;
  tierLightCpu: string;
  tierLightRam: string;
  tierLightStorage: string;
  tierLightPower: string;
  tierMediaTitle: string;
  tierMediaDesc: string;
  tierMediaCpu: string;
  tierMediaRam: string;
  tierMediaStorage: string;
  tierMediaPower: string;
  tierAITitle: string;
  tierAIDesc: string;
  tierAICpu: string;
  tierAIRam: string;
  tierAIStorage: string;
  tierAIPower: string;
  calcTitle: string;
  calcSubtitle: string;
  featureDMSLabel: string;
  featureAudioLabel: string;
  featureVideoLabel: string;
  featureDiscLabel: string;
  featureAILabel: string;
  recommendedTierLabel: string;
  turnkeyTitle: string;
  turnkeySubtitle: string;
  turnkeyPreorderBtn: string;
  turnkeyReservedNotice: string;
  turnkeyModalTitle: string;
  turnkeyEmailLabel: string;
  turnkeySubmitBtn: string;

  // Landing Marketing & Showcase (Story 7.2)
  landingTitle: string;
  landingHeroHeadline: string;
  landingHeroSubheadline: string;
  landingCtaDashboard: string;
  landingCtaGitHub: string;
  landingCtaHardware: string;
  landingFeatureShowcaseTitle: string;
  landingFeatureShowcaseSubtitle: string;
  landingFeatureVideoTitle: string;
  landingFeatureVideoDesc: string;
  landingFeatureAudioTitle: string;
  landingFeatureAudioDesc: string;
  landingFeatureDMSTitle: string;
  landingFeatureDMSDesc: string;
  landingFeatureAITitle: string;
  landingFeatureAIDesc: string;
  landingFeatureDiscTitle: string;
  landingFeatureDiscDesc: string;
  landingPrivacyTitle: string;
  landingPrivacySubtitle: string;
  landingPrivacyPill1Title: string;
  landingPrivacyPill1Desc: string;
  landingPrivacyPill2Title: string;
  landingPrivacyPill2Desc: string;
  landingPrivacyPill3Title: string;
  landingPrivacyPill3Desc: string;
  landingPrivacyPill4Title: string;
  landingPrivacyPill4Desc: string;
  landingOpenSourceTitle: string;
  landingOpenSourceSubtitle: string;
  landingNavLanding: string;
  landingHeroBadge: string;
  landingStatPings: string;
  landingStatStorage: string;
  landingStatSpokes: string;
  landingStatPower: string;
  landingBadgeVideo: string;
  landingBadgeAudio: string;
  landingBadgeDms: string;
  landingBadgeAi: string;
  landingBadgeDisc: string;
  landingReadGuideMedia: string;
  landingReadGuideAudio: string;
  landingReadGuideDms: string;
  landingReadGuideAi: string;
  landingReadGuideDisc: string;
  landingFullHardwareGuides: string;
  landingFooterBrandDesc: string;
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
    guidesTitle: "Appliance Guides & Help",
    guidesSubtitle: "Step-by-step documentation for setting up, managing, and maintaining your Sovereign home server.",
    guideQuickstart: "Quickstart & Setup",
    guideStorage: "Storage & Network NAS",
    guideMedia: "Media & Dropzone",
    guideAI: "Private Local AI",
    guideResilience: "Power & Rollback Safety",
    bootSafetyChecked: "Checking system safety after power restoration... your files are secure.",
    bootSafetyVerifying: "Verifying system integrity and storage safety before launching services.",
    bootSafetySuccess: "System safety verified. Redirecting to dashboard...",
    bootSafetySuccessDesc: "All storage volumes and database integrity checks completed successfully.",
    bootSafetyError: "System verification detected an issue. Your files are protected in safety mode. Automated restore recommended.",
    bootSafetyErrorDesc: "Your data remains safe and intact on local storage. No action is required while the system stabilizes.",
    bootStorageDegraded: "Hardware attention needed: A hard drive is reporting errors. Your existing files are locked in read-only safety mode.",
    bootBadgeChecking: "Verifying Storage",
    bootBadgeSuccess: "System Operational",
    bootBadgeAttention: "Attention Required",
    storageIngestionReady: "Storage Ingestion Ready",
    destinationLibrary: "Destination Library",
    mediaCategoryTabs: "Media Category Tabs",
    daytimeSchedule: "☀️ Daytime (AI Priority): 07:00 – 01:00",
    nighttimeSchedule: "🌙 Nighttime (Batch Transcode): 01:00 – 07:00",
    interactiveAILabel: "Interactive AI (18h)",
    batchTranscodeLabel: "Batch (6h)",
    balanceHardwareSharing: "Balance real-time AI responsiveness against heavy batch video transcoding.",
    systemMaintenanceDesc: "System maintenance, updates, and storage diagnostic health.",
    dailyUpdateWindow: "Daily Update Window",
    rollbackProtectionLabel: "Rollback Protection",
    rollbackProtectionActive: "Active (180s probe window)",
    applianceHostname: "Appliance Hostname",
    destructiveControlsTitle: "Destructive Appliance Controls",
    destructiveControlsDesc: "Administrative operations requiring explicit confirmation. Safeguards prevent accidental execution.",
    hardwareTitle: "Hardware Guide & Tiers",
    hardwareSubtitle: "Choose or assemble the ideal hardware profile for your household workloads, or explore turnkey pre-assembled appliances.",
    tierLightTitle: "Light Tier",
    tierLightDesc: "Document archiving, OCR text search, and lossless music streaming. Ultra-low power, silent, and fanless.",
    tierLightCpu: "Dual-core x86 or 64-bit ARM (Intel N100, Celeron, RPi 5)",
    tierLightRam: "4 GB DDR4 / DDR5",
    tierLightStorage: "64 GB SSD (OS) + 250 GB–1 TB storage",
    tierLightPower: "5W – 10W idle",
    tierMediaTitle: "Media Tier",
    tierMediaDesc: "4K video streaming with Intel QuickSync tone mapping, multi-room audio, and automatic optical disc ingestion.",
    tierMediaCpu: "Quad-core x86 with Intel QuickSync iGPU (N100, Core i3/i5)",
    tierMediaRam: "8 GB DDR4 / DDR5",
    tierMediaStorage: "2 TB – 8 TB SATA / NVMe pool",
    tierMediaPower: "10W – 25W idle",
    tierAITitle: "AI & Power Tier",
    tierAIDesc: "Private household large language model inference (llama.cpp 7B/8B), zero-vector FTS5 RAG search, and full media streaming.",
    tierAICpu: "6+ core modern x86 CPU (Intel Core i5/i7, AMD Ryzen 7)",
    tierAIRam: "16 GB – 32 GB RAM + 8–16 GB VRAM GPU",
    tierAIStorage: "4 TB – 16 TB high-speed NVMe / SATA pool",
    tierAIPower: "25W – 45W idle",
    calcTitle: "Interactive Hardware Tier Calculator",
    calcSubtitle: "Select the features you plan to run to calculate your optimal hardware tier and resource targets.",
    featureDMSLabel: "Document Archiving & OCR (Paperless-ngx)",
    featureAudioLabel: "Music & Audiobooks (Navidrome & Audiobookshelf)",
    featureVideoLabel: "4K Video Streaming & Transcoding (Jellyfin)",
    featureDiscLabel: "Automatic Optical Disc Ripping (ARM)",
    featureAILabel: "Local Private AI Chat & Document RAG (llama.cpp)",
    recommendedTierLabel: "Recommended Profile",
    turnkeyTitle: "Turnkey Pre-Assembled Sovereign Appliances",
    turnkeySubtitle: "Prefer an elegant, silent plug-and-play appliance? Reserve priority manufacturing slots for official factory-built editions.",
    turnkeyPreorderBtn: "Reserve Turnkey Appliance",
    turnkeyReservedNotice: "Reservation confirmed! Your manufacturing token has been recorded.",
    turnkeyModalTitle: "Priority Hardware Reservation",
    turnkeyEmailLabel: "Email for notification",
    turnkeySubmitBtn: "Confirm Priority Reservation",
    landingTitle: "Sovereign — Private Home Server",
    landingHeroHeadline: "Your Personal Digital Sanctuary",
    landingHeroSubheadline: "100% private, zero-telemetry home server appliance. Stream 4K media, archive documents with OCR, run local AI, and ingest optical discs on hardware you own.",
    landingCtaDashboard: "Open Appliance Dashboard",
    landingCtaGitHub: "View on GitHub",
    landingCtaHardware: "Hardware Calculator",
    landingFeatureShowcaseTitle: "Integrated Private Household Services",
    landingFeatureShowcaseSubtitle: "Five pre-configured spoke appliances unified under a single private dashboard, running completely within your home network.",
    landingFeatureVideoTitle: "4K Video Streaming & Transcoding",
    landingFeatureVideoDesc: "Hardware-accelerated video streaming powered by Jellyfin. Intel QuickSync tone mapping enables smooth, stutter-free playback on all family devices.",
    landingFeatureAudioTitle: "Lossless Music & Audiobooks",
    landingFeatureAudioDesc: "Navidrome provides pristine FLAC and MP3 streaming, while Audiobookshelf syncs listening progress, chapters, and bookmarks across user accounts.",
    landingFeatureDMSTitle: "Intelligent Document Archiving",
    landingFeatureDMSDesc: "Paperless-ngx delivers automated OCR text indexing, multi-channel email ingestion, and intelligent deadline detection for household contracts and bills.",
    landingFeatureAITitle: "Local Private AI & Instant RAG",
    landingFeatureAIDesc: "Local llama.cpp 7B/8B GGUF inference running on an air-gapped network. Query your personal document archive with zero cloud telemetry exposure.",
    landingFeatureDiscTitle: "Automated Optical Disc Ingestion",
    landingFeatureDiscDesc: "Automatic Ripping Machine (ARM) detects audio CDs, DVDs, and Blu-rays on insertion, losslessly rips media, queries poster artwork, and ejects.",
    landingPrivacyTitle: "Zero-Telemetry Architectural Invariants",
    landingPrivacySubtitle: "Engineered from the ground up to guarantee absolute family privacy and high operational resilience.",
    landingPrivacyPill1Title: "Air-Gapped AI Network",
    landingPrivacyPill1Desc: "AI backend runs on an isolated internal Docker bridge without internet routing. Prompts and documents never leave your physical appliance.",
    landingPrivacyPill2Title: "Zero Docker Socket Exposure",
    landingPrivacyPill2Desc: "Spoke application containers have zero access to the host Docker daemon socket, eliminating container breakout vulnerabilities.",
    landingPrivacyPill3Title: "CGO-Free WAL Durability",
    landingPrivacyPill3Desc: "Pure Go SQLite with Write-Ahead Logging ensures database integrity and instant crash recovery during sudden power outages.",
    landingPrivacyPill4Title: "Atomic 180s Rollback Safety",
    landingPrivacyPill4Desc: "Scheduled container updates probe service health for 180 seconds and revert atomically to prior snapshots if errors occur.",
    landingOpenSourceTitle: "100% Open-Source & Community-Driven",
    landingOpenSourceSubtitle: "Built on battle-tested open-source software using the BMad Method. Inspect code, audit security, and assemble your own hardware freely.",
    landingNavLanding: "Showcase",
    landingHeroBadge: "100% Private · Zero Cloud Telemetry · Appliance Grade",
    landingStatPings: "Cloud Pings",
    landingStatStorage: "Local Storage",
    landingStatSpokes: "Unified Spokes",
    landingStatPower: "Idle Capable",
    landingBadgeVideo: "⚡ QuickSync Hardware Accelerated",
    landingBadgeAudio: "🎧 Lossless FLAC & Multi-User Audiobooks",
    landingBadgeDms: "📑 Automated OCR & Deadline Extraction",
    landingBadgeAi: "🤖 Local 7B/8B LLM & Document RAG",
    landingBadgeDisc: "💿 Insert & Walk Away Ingestion",
    landingReadGuideMedia: "Read Media Setup Guide",
    landingReadGuideAudio: "Read Audio Setup Guide",
    landingReadGuideDms: "Read Document Archival Guide",
    landingReadGuideAi: "Read Local AI Guide",
    landingReadGuideDisc: "Read Optical Ripping Guide",
    landingFullHardwareGuides: "Full Hardware Specification Guides & Hardware Tier Roadmap",
    landingFooterBrandDesc: "100% Private Self-Hosted Home Server",
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
    guidesTitle: "Anleitungen & Hilfe",
    guidesSubtitle: "Schritt-für-Schritt-Anleitungen zur Einrichtung, Verwaltung und Wartung Ihres Sovereign-Heimservers.",
    guideQuickstart: "Schnellstart & Einrichtung",
    guideStorage: "Speicher & Netzwerk-NAS",
    guideMedia: "Medien & Dropzone",
    guideAI: "Private lokale KI",
    guideResilience: "Ausfallsicherheit & Rollback",
    bootSafetyChecked: "Systemsicherheit wird nach Stromwiederherstellung überprüft... Ihre Dateien sind sicher.",
    bootSafetyVerifying: "Systemintegrität und Speichersicherheit werden vor dem Starten der Dienste überprüft.",
    bootSafetySuccess: "Systemsicherheit bestätigt. Weiterleitung zur Übersicht...",
    bootSafetySuccessDesc: "Alle Speicherbereiche und Datenbank-Integritätsprüfungen erfolgreich abgeschlossen.",
    bootSafetyError: "Systemprüfung hat ein Problem festgestellt. Ihre Dateien sind im Sicherheitsmodus geschützt. Automatische Wiederherstellung empfohlen.",
    bootSafetyErrorDesc: "Ihre Daten bleiben sicher und intakt auf dem lokalen Speicher. Keine Aktion erforderlich, während sich das System stabilisiert.",
    bootStorageDegraded: "Hardware-Aufmerksamkeit erforderlich: Eine Festplatte meldet Fehler. Ihre bestehenden Dateien sind im schreibgeschützten Sicherheitsmodus gesperrt.",
    bootBadgeChecking: "Speicher wird geprüft",
    bootBadgeSuccess: "System betriebsbereit",
    bootBadgeAttention: "Aufmerksamkeit erforderlich",
    storageIngestionReady: "Speicheraufnahme bereit",
    destinationLibrary: "Zielbibliothek",
    mediaCategoryTabs: "Medienkategorie-Reiter",
    daytimeSchedule: "☀️ Tagsüber (KI-Priorität): 07:00 – 01:00",
    nighttimeSchedule: "🌙 Nachts (Stapel-Transkodierung): 01:00 – 07:00",
    interactiveAILabel: "Interaktive KI (18h)",
    batchTranscodeLabel: "Stapelverarbeitung (6h)",
    balanceHardwareSharing: "Echtzeit-KI-Reaktionsfähigkeit mit Video-Stapeltranskodierung ausbalancieren.",
    systemMaintenanceDesc: "Systemwartung, Aktualisierungen und Speicherdiagnose.",
    dailyUpdateWindow: "Tägliches Update-Fenster",
    rollbackProtectionLabel: "Rollback-Schutz",
    rollbackProtectionActive: "Aktiv (180s Prüffenster)",
    applianceHostname: "Geräte-Hostname",
    destructiveControlsTitle: "Destruktive Gerätesteuerung",
    destructiveControlsDesc: "Verwaltungsvorgänge mit expliziter Bestätigung. Sicherheitsmechanismen verhindern versehentliche Ausführung.",
    hardwareTitle: "Hardware-Leitfaden & Stufen",
    hardwareSubtitle: "Wählen oder montieren Sie das ideale Hardware-Profil für Ihre Haushaltsanforderungen oder entdecken Sie vorkonfigurierte Komplettsysteme.",
    tierLightTitle: "Light-Stufe",
    tierLightDesc: "Dokumentenarchivierung, OCR-Textsuche und verlustfreies Musik-Streaming. Extrem sparsam, leise und lüfterlos.",
    tierLightCpu: "Dual-Core x86 oder 64-Bit ARM (Intel N100, Celeron, RPi 5)",
    tierLightRam: "4 GB DDR4 / DDR5",
    tierLightStorage: "64 GB SSD (Betriebssystem) + 250 GB–1 TB Speicher",
    tierLightPower: "5W – 10W Leerlauf",
    tierMediaTitle: "Medien-Stufe",
    tierMediaDesc: "4K-Video-Streaming mit Intel QuickSync, Multiroom-Audio und automatischem Einlesen optischer Datenträger.",
    tierMediaCpu: "Quad-Core x86 mit Intel QuickSync iGPU (N100, Core i3/i5)",
    tierMediaRam: "8 GB DDR4 / DDR5",
    tierMediaStorage: "2 TB – 8 TB SATA / NVMe Speicherpool",
    tierMediaPower: "10W – 25W Leerlauf",
    tierAITitle: "KI & Power-Stufe",
    tierAIDesc: "Lokale Haushalts-KI mit 7B/8B LLM-Inferenz (llama.cpp), FTS5-RAG-Suche und vollem Medien-Streaming.",
    tierAICpu: "6+ Kerne moderner x86 CPU (Intel Core i5/i7, AMD Ryzen 7)",
    tierAIRam: "16 GB – 32 GB RAM + 8–16 GB VRAM GPU",
    tierAIStorage: "4 TB – 16 TB High-Speed NVMe / SATA Speicherpool",
    tierAIPower: "25W – 45W Leerlauf",
    calcTitle: "Interaktiver Hardware-Stufenrechner",
    calcSubtitle: "Wählen Sie die gewünschten Funktionen aus, um Ihre optimale Hardware-Stufe und Zielkonfiguration zu ermitteln.",
    featureDMSLabel: "Dokumentenarchivierung & OCR (Paperless-ngx)",
    featureAudioLabel: "Musik & Hörbücher (Navidrome & Audiobookshelf)",
    featureVideoLabel: "4K-Video-Streaming & Transkodierung (Jellyfin)",
    featureDiscLabel: "Automatisches Einlesen von CDs/DVDs/Blu-rays (ARM)",
    featureAILabel: "Lokaler privater KI-Chat & Dokumenten-RAG (llama.cpp)",
    recommendedTierLabel: "Empfohlenes Profil",
    turnkeyTitle: "Vorkonfigurierte Sovereign-Komplettgeräte",
    turnkeySubtitle: "Bevorzugen Sie ein elegantes, geräuschloses Plug-and-Play-Gerät? Reservieren Sie Prioritäts-Fertigungsplätze für offizielle Editionen.",
    turnkeyPreorderBtn: "Komplettgerät reservieren",
    turnkeyReservedNotice: "Reservierung bestätigt! Ihr Fertigungstoken wurde gespeichert.",
    turnkeyModalTitle: "Prioritäre Hardware-Reservierung",
    turnkeyEmailLabel: "E-Mail für Benachrichtigung",
    turnkeySubmitBtn: "Prioritäts-Reservierung bestätigen",
    landingTitle: "Sovereign — Privater Heimserver",
    landingHeroHeadline: "Ihr persönlicher digitaler Zufluchtsort",
    landingHeroSubheadline: "100 % private, telemetriefreie Heimserver-Appliance. Streamen Sie 4K-Medien, archivieren Sie Dokumente mit OCR, nutzen Sie lokale KI und lesen Sie optische Discs ein – auf Hardware, die Ihnen gehört.",
    landingCtaDashboard: "Appliance-Dashboard öffnen",
    landingCtaGitHub: "Auf GitHub ansehen",
    landingCtaHardware: "Hardware-Rechner",
    landingFeatureShowcaseTitle: "Integrierte private Haushaltsdienste",
    landingFeatureShowcaseSubtitle: "Fünf vorkonfigurierte Dienste unter einem einzigen privaten Dashboard, die vollständig in Ihrem Heimnetzwerk betrieben werden.",
    landingFeatureVideoTitle: "4K-Video-Streaming & Transkodierung",
    landingFeatureVideoDesc: "Hardwarebeschleunigtes Streaming mit Jellyfin. Intel QuickSync Tone-Mapping ermöglicht flüssige Wiedergabe auf allen Familiengeräten.",
    landingFeatureAudioTitle: "Verlustfreie Musik & Hörbücher",
    landingFeatureAudioDesc: "Navidrome liefert hochauflösendes FLAC- und MP3-Streaming, während Audiobookshelf Hörfortschritte und Lesezeichen synchronisiert.",
    landingFeatureDMSTitle: "Intelligente Dokumentenarchivierung",
    landingFeatureDMSDesc: "Paperless-ngx bietet automatische OCR-Texterkennung, Mehrkanal-E-Mail-Polling und automatische Fristenerkennung für Verträge und Rechnungen.",
    landingFeatureAITitle: "Lokale private KI & Dokumenten-RAG",
    landingFeatureAIDesc: "Lokale llama.cpp 7B/8B GGUF-Inferenz im abgeschotteten Netzwerk. Befragen Sie Ihr persönliches Dokumentenarchiv ohne Cloud-Telemetrie.",
    landingFeatureDiscTitle: "Automatisches Einlesen von Discs",
    landingFeatureDiscDesc: "Die Automatic Ripping Machine (ARM) erkennt Musik-CDs, DVDs und Blu-rays beim Einlegen, rippt verlustfrei, lädt Cover und wirft die Disc aus.",
    landingPrivacyTitle: "Telemetriefreie Architektur-Garantien",
    landingPrivacySubtitle: "Von Grund auf entwickelt, um absolute Privatsphäre und maximale Ausfallsicherheit zu gewährleisten.",
    landingPrivacyPill1Title: "Abgeschottetes KI-Netzwerk",
    landingPrivacyPill1Desc: "Das KI-Backend läuft auf einer isolierten internen Docker-Bridge ohne Internet-Routing. Prompts und Daten verlassen niemals Ihr Haus.",
    landingPrivacyPill2Title: "Keine Docker-Socket-Freigabe",
    landingPrivacyPill2Desc: "Anwendungs-Container haben keinerlei Zugriff auf den Docker-Socket des Host-Systems, was Sicherheitsausbrüche verhindert.",
    landingPrivacyPill3Title: "CGO-freie WAL-Datenbank",
    landingPrivacyPill3Desc: "Reines Go-SQLite mit Write-Ahead Logging garantiert Datenbankintegrität und sofortige Wiederherstellung nach Stromausfällen.",
    landingPrivacyPill4Title: "Atomare 180s-Rollback-Sicherheit",
    landingPrivacyPill4Desc: "Nächtliche Container-Updates prüfen die Dienstgesundheit 180 Sekunden lang und rollen bei Fehlern atomar auf Backups zurück.",
    landingOpenSourceTitle: "100 % Open Source & Community-geführt",
    landingOpenSourceSubtitle: "Aufgebaut auf bewährten Open-Source-Grundlagen nach der BMad-Methode. Prüfen Sie den Code und bauen Sie eigene Hardware.",
    landingNavLanding: "Showcase",
    landingHeroBadge: "100 % privat · Keine Cloud-Telemetrie · Appliance-Standard",
    landingStatPings: "Cloud-Pings",
    landingStatStorage: "Lokaler Speicher",
    landingStatSpokes: "Integrierte Dienste",
    landingStatPower: "Leerlaufleistung",
    landingBadgeVideo: "⚡ Hardware-beschleunigtes QuickSync",
    landingBadgeAudio: "🎧 Verlustfreies FLAC & Hörbücher für mehrere Benutzer",
    landingBadgeDms: "📑 Automatische OCR & Fristenerkennung",
    landingBadgeAi: "🤖 Lokales 7B/8B LLM & Dokumenten-RAG",
    landingBadgeDisc: "💿 Automatisches Einlesen von Discs",
    landingReadGuideMedia: "Medien-Leitfaden lesen",
    landingReadGuideAudio: "Audio-Leitfaden lesen",
    landingReadGuideDms: "Dokumentenarchivierungs-Leitfaden lesen",
    landingReadGuideAi: "Lokalen KI-Leitfaden lesen",
    landingReadGuideDisc: "Disc-Ripping-Leitfaden lesen",
    landingFullHardwareGuides: "Vollständige Hardware-Leitfäden & Stufen-Roadmap",
    landingFooterBrandDesc: "100 % privater selbstgehosteter Heimserver",
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
    guidesTitle: "Guías de ayuda del electrodoméstico",
    guidesSubtitle: "Documentación paso a paso para configurar, gestionar y mantener su servidor doméstico Sovereign.",
    guideQuickstart: "Inicio rápido y configuración",
    guideStorage: "Almacenamiento y NAS en red",
    guideMedia: "Medios y zona de entrega",
    guideAI: "IA local y privada",
    guideResilience: "Resiliencia eléctrica y restauración",
    bootSafetyChecked: "Comprobando la seguridad del sistema tras la restauración eléctrica... sus archivos están seguros.",
    bootSafetyVerifying: "Verificando la integridad del sistema y la seguridad del almacenamiento antes de iniciar servicios.",
    bootSafetySuccess: "Seguridad del sistema verificada. Redirigiendo al panel...",
    bootSafetySuccessDesc: "Todas las comprobaciones de integridad de almacenamiento y base de datos se completaron con éxito.",
    bootSafetyError: "La verificación del sistema detectó un problema. Sus archivos están protegidos en modo seguro. Se recomienda restauración automática.",
    bootSafetyErrorDesc: "Sus datos permanecen seguros e intactos en el almacenamiento local. No se requiere ninguna acción mientras el sistema se estabiliza.",
    bootStorageDegraded: "Atención de hardware necesaria: Un disco duro está reportando errores. Sus archivos están bloqueados en modo seguro de solo lectura.",
    bootBadgeChecking: "Verificando almacenamiento",
    bootBadgeSuccess: "Sistema operativo",
    bootBadgeAttention: "Atención requerida",
    storageIngestionReady: "Ingesta de almacenamiento lista",
    destinationLibrary: "Biblioteca de destino",
    mediaCategoryTabs: "Pestañas de categoría multimedia",
    daytimeSchedule: "☀️ Día (Prioridad IA): 07:00 – 01:00",
    nighttimeSchedule: "🌙 Noche (Transcodificación por lotes): 01:00 – 07:00",
    interactiveAILabel: "IA interactiva (18h)",
    batchTranscodeLabel: "Lotes (6h)",
    balanceHardwareSharing: "Equilibrar la capacidad de respuesta de la IA en tiempo real frente a la transcodificación de vídeo por lotes.",
    systemMaintenanceDesc: "Mantenimiento del sistema, actualizaciones y salud diagnóstica del almacenamiento.",
    dailyUpdateWindow: "Ventana de actualización diaria",
    rollbackProtectionLabel: "Protección de reversión",
    rollbackProtectionActive: "Activa (ventana de sondeo de 180s)",
    applianceHostname: "Nombre de host del dispositivo",
    destructiveControlsTitle: "Controles destructivos del dispositivo",
    destructiveControlsDesc: "Operaciones administrativas que requieren confirmación explícita. Las protecciones evitan la ejecución accidental.",
    hardwareTitle: "Guía y niveles de hardware",
    hardwareSubtitle: "Elija o monte el perfil de hardware ideal para las cargas de trabajo de su hogar, o explore dispositivos preensamblados listos para usar.",
    tierLightTitle: "Nivel Ligero",
    tierLightDesc: "Archivo de documentos, búsqueda de texto OCR y streaming de música sin pérdidas. Consumo ultrabajo, silencioso y sin ventilador.",
    tierLightCpu: "Doble núcleo x86 o ARM de 64 bits (Intel N100, Celeron, RPi 5)",
    tierLightRam: "4 GB DDR4 / DDR5",
    tierLightStorage: "64 GB SSD (SO) + 250 GB–1 TB almacenamiento",
    tierLightPower: "5W – 10W en reposo",
    tierMediaTitle: "Nivel Multimedia",
    tierMediaDesc: "Streaming de vídeo 4K con Intel QuickSync, audio multiroom e ingesta automática de discos ópticos.",
    tierMediaCpu: "Cuatro núcleos x86 con iGPU Intel QuickSync (N100, Core i3/i5)",
    tierMediaRam: "8 GB DDR4 / DDR5",
    tierMediaStorage: "2 TB – 8 TB grupo SATA / NVMe",
    tierMediaPower: "10W – 25W en reposo",
    tierAITitle: "Nivel IA y Potencia",
    tierAIDesc: "Inferencia de modelos de lenguaje locales (llama.cpp 7B/8B), búsqueda RAG FTS5 y streaming completo de medios.",
    tierAICpu: "CPU moderna x86 de 6+ núcleos (Intel Core i5/i7, AMD Ryzen 7)",
    tierAIRam: "16 GB – 32 GB RAM + GPU con 8–16 GB VRAM",
    tierAIStorage: "4 TB – 16 TB grupo de alta velocidad NVMe / SATA",
    tierAIPower: "25W – 45W en reposo",
    calcTitle: "Calculadora interactiva de niveles de hardware",
    calcSubtitle: "Seleccione las funciones que planea ejecutar para calcular su nivel de hardware óptimo y los recursos necesarios.",
    featureDMSLabel: "Archivo de documentos y OCR (Paperless-ngx)",
    featureAudioLabel: "Música y audiolibros (Navidrome y Audiobookshelf)",
    featureVideoLabel: "Streaming y transcodificación de vídeo 4K (Jellyfin)",
    featureDiscLabel: "Extracción automática de discos ópticos (ARM)",
    featureAILabel: "Chat de IA privada local y RAG de documentos (llama.cpp)",
    recommendedTierLabel: "Perfil recomendado",
    turnkeyTitle: "Dispositivos Sovereign preensamblados listos para usar",
    turnkeySubtitle: "¿Prefiere un dispositivo elegante y silencioso listo para conectar? Reserve puestos prioritarios de fabricación para las ediciones oficiales.",
    turnkeyPreorderBtn: "Reservar dispositivo listo para usar",
    turnkeyReservedNotice: "¡Reserva confirmada! Su token de fabricación ha sido registrado.",
    turnkeyModalTitle: "Reserva prioritaria de hardware",
    turnkeyEmailLabel: "Correo para notificación",
    turnkeySubmitBtn: "Confirmar reserva prioritaria",
    landingTitle: "Sovereign — Servidor doméstico privado",
    landingHeroHeadline: "Su santuario digital personal",
    landingHeroSubheadline: "Dispositivo de servidor doméstico 100% privado y sin telemetría. Transmita medios en 4K, archive documentos con OCR, ejecute IA local y extraiga discos ópticos en hardware de su propiedad.",
    landingCtaDashboard: "Abrir panel de control",
    landingCtaGitHub: "Ver en GitHub",
    landingCtaHardware: "Calculadora de hardware",
    landingFeatureShowcaseTitle: "Servicios domésticos privados integrados",
    landingFeatureShowcaseSubtitle: "Cinco servicios preconfigurados unificados bajo un único panel de control privado, ejecutados en su red doméstica.",
    landingFeatureVideoTitle: "Streaming y transcodificación de vídeo 4K",
    landingFeatureVideoDesc: "Streaming con aceleración de hardware con Jellyfin. El mapeo de tonos Intel QuickSync permite una reproducción fluida en todos los dispositivos.",
    landingFeatureAudioTitle: "Música sin pérdidas y audiolibros",
    landingFeatureAudioDesc: "Navidrome ofrece streaming FLAC y MP3 de alta fidelidad, mientras Audiobookshelf sincroniza capítulos y marcas de lectura.",
    landingFeatureDMSTitle: "Gestión documental inteligente",
    landingFeatureDMSDesc: "Paperless-ngx proporciona indexación de texto OCR, ingesta automática por correo y detección de plazos de pago y cancelación.",
    landingFeatureAITitle: "IA privada local y RAG instantáneo",
    landingFeatureAIDesc: "Inferencia GGUF 7B/8B local con llama.cpp en red aislada. Consulte su archivo de documentos personales sin enviar datos a la nube.",
    landingFeatureDiscTitle: "Extracción automática de discos ópticos",
    landingFeatureDiscDesc: "Automatic Ripping Machine (ARM) detecta CD, DVD y Blu-ray al insertarlos, extrae el contenido sin pérdidas y expulsa el disco.",
    landingPrivacyTitle: "Garantías arquitectónicas sin telemetría",
    landingPrivacySubtitle: "Diseñado desde el principio para garantizar la privacidad familiar y la resiliencia operativa.",
    landingPrivacyPill1Title: "Red de IA aislada (Air-Gapped)",
    landingPrivacyPill1Desc: "El backend de IA se ejecuta en un puente Docker interno sin enrutamiento a internet. Nada sale de su dispositivo físico.",
    landingPrivacyPill2Title: "Cero montajes del socket Docker",
    landingPrivacyPill2Desc: "Los contenedores de aplicaciones no tienen acceso al socket de Docker del host, previniendo vulnerabilidades de escalada.",
    landingPrivacyPill3Title: "Durabilidad WAL en SQLite puro",
    landingPrivacyPill3Desc: "SQLite en Go puro con Write-Ahead Logging asegura integridad de datos y recuperación instantánea tras cortes de energía.",
    landingPrivacyPill4Title: "Seguridad de reversión atómica de 180s",
    landingPrivacyPill4Desc: "Las actualizaciones programadas verifican el estado durante 180 segundos y revierten automáticamente ante cualquier fallo.",
    landingOpenSourceTitle: "100% Código Abierto e Impulsado por la Comunidad",
    landingOpenSourceSubtitle: "Construido sobre pilares de código abierto mediante el Método BMad. Audite el código y configure su propio hardware libremente.",
    landingNavLanding: "Presentación",
    landingHeroBadge: "100% privado · Cero telemetría en la nube · Nivel electrodoméstico",
    landingStatPings: "Pings a la nube",
    landingStatStorage: "Almacenamiento local",
    landingStatSpokes: "Servicios integrados",
    landingStatPower: "Consumo en reposo",
    landingBadgeVideo: "⚡ QuickSync acelerado por hardware",
    landingBadgeAudio: "🎧 FLAC sin pérdidas y audiolibros multiusuario",
    landingBadgeDms: "📑 OCR automático y detección de plazos",
    landingBadgeAi: "🤖 LLM local 7B/8B y RAG de documentos",
    landingBadgeDisc: "💿 Extracción automática de discos ópticos",
    landingReadGuideMedia: "Leer guía de configuración de medios",
    landingReadGuideAudio: "Leer guía de audio",
    landingReadGuideDms: "Leer guía de archivo de documentos",
    landingReadGuideAi: "Leer guía de IA local",
    landingReadGuideDisc: "Leer guía de digitalización de discos",
    landingFullHardwareGuides: "Guías completas de especificaciones de hardware y mapa de niveles",
    landingFooterBrandDesc: "Servidor doméstico 100% privado y autónomo",
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
    guidesTitle: "Guides et aide de l'appareil",
    guidesSubtitle: "Documentation pas à pas pour configurer, gérer et maintenir votre serveur domestique Sovereign.",
    guideQuickstart: "Démarrage rapide & Configuration",
    guideStorage: "Stockage & NAS réseau",
    guideMedia: "Médias & Zone de dépôt",
    guideAI: "IA locale et privée",
    guideResilience: "Résilience électrique & Restauration",
    bootSafetyChecked: "Vérification de la sécurité du système après rétablissement du courant... vos fichiers sont en sécurité.",
    bootSafetyVerifying: "Vérification de l'intégrité du système et de la sécurité du stockage avant le lancement des services.",
    bootSafetySuccess: "Sécurité du système vérifiée. Redirection vers le tableau de bord...",
    bootSafetySuccessDesc: "Tous les volumes de stockage et contrôles d'intégrité de la base de données ont réussi.",
    bootSafetyError: "La vérification du système a détecté un problème. Vos fichiers sont protégés en mode sans échec. Restauration automatique recommandée.",
    bootSafetyErrorDesc: "Vos données restent en sécurité et intactes sur le stockage local. Aucune action requise pendant la stabilisation du système.",
    bootStorageDegraded: "Attention matérielle requise : Un disque dur signale des erreurs. Vos fichiers sont verrouillés en mode lecture seule sécurisé.",
    bootBadgeChecking: "Vérification du stockage",
    bootBadgeSuccess: "Système opérationnel",
    bootBadgeAttention: "Attention requise",
    storageIngestionReady: "Ingestion de stockage prête",
    destinationLibrary: "Bibliothèque de destination",
    mediaCategoryTabs: "Onglets des catégories multimédias",
    daytimeSchedule: "☀️ Journée (Priorité IA) : 07:00 – 01:00",
    nighttimeSchedule: "🌙 Nuit (Transcodage par lots) : 01:00 – 07:00",
    interactiveAILabel: "IA interactive (18h)",
    batchTranscodeLabel: "Lots (6h)",
    balanceHardwareSharing: "Équilibrer la réactivité de l'IA en temps réel avec le transcodage vidéo par lots.",
    systemMaintenanceDesc: "Maintenance du système, mises à jour et intégrité diagnostique du stockage.",
    dailyUpdateWindow: "Fenêtre de mise à jour quotidienne",
    rollbackProtectionLabel: "Protection contre le retour en arrière",
    rollbackProtectionActive: "Active (fenêtre de test de 180s)",
    applianceHostname: "Nom d'hôte de l'appareil",
    destructiveControlsTitle: "Contrôles destructifs de l'appareil",
    destructiveControlsDesc: "Opérations administratives nécessitant une confirmation explicite. Des sécurités empêchent l'exécution accidentelle.",
    hardwareTitle: "Guide et profils matériels",
    hardwareSubtitle: "Choisissez ou assemblez le profil matériel idéal pour les besoins de votre foyer, ou découvrez les appareils clés en main préassemblés.",
    tierLightTitle: "Profil Léger",
    tierLightDesc: "Archivage de documents, recherche textuelle OCR et streaming musical sans perte. Très faible consommation, silencieux et sans ventilateur.",
    tierLightCpu: "Double cœur x86 ou ARM 64 bits (Intel N100, Celeron, RPi 5)",
    tierLightRam: "4 Go DDR4 / DDR5",
    tierLightStorage: "64 Go SSD (OS) + 250 Go–1 To stockage",
    tierLightPower: "5W – 10W au repos",
    tierMediaTitle: "Profil Médias",
    tierMediaDesc: "Streaming vidéo 4K avec Intel QuickSync, audio multi-pièces et ingestion automatique de disques optiques.",
    tierMediaCpu: "Quatre cœurs x86 avec iGPU Intel QuickSync (N100, Core i3/i5)",
    tierMediaRam: "8 Go DDR4 / DDR5",
    tierMediaStorage: "2 To – 8 To pool SATA / NVMe",
    tierMediaPower: "10W – 25W au repos",
    tierAITitle: "Profil IA et Puissance",
    tierAIDesc: "Inférence de modèles de langage locaux (llama.cpp 7B/8B), recherche RAG FTS5 et streaming multimédia complet.",
    tierAICpu: "CPU x86 moderne 6+ cœurs (Intel Core i5/i7, AMD Ryzen 7)",
    tierAIRam: "16 Go – 32 Go RAM + GPU avec 8–16 Go VRAM",
    tierAIStorage: "4 To – 16 To pool haute vitesse NVMe / SATA",
    tierAIPower: "25W – 45W au repos",
    calcTitle: "Calculateur interactif de profils matériels",
    calcSubtitle: "Sélectionnez les fonctionnalités souhaitées pour déterminer votre profil matériel optimal et les ressources nécessaires.",
    featureDMSLabel: "Archivage de documents et OCR (Paperless-ngx)",
    featureAudioLabel: "Musique et livres audio (Navidrome & Audiobookshelf)",
    featureVideoLabel: "Streaming et transcodage vidéo 4K (Jellyfin)",
    featureDiscLabel: "Numérisation automatique de disques optiques (ARM)",
    featureAILabel: "Chat IA privée locale et RAG de documents (llama.cpp)",
    recommendedTierLabel: "Profil recommandé",
    turnkeyTitle: "Appareils Sovereign préassemblés prêts à l'emploi",
    turnkeySubtitle: "Vous préférez un appareil élégant et silencieux prêt à brancher ? Réservez des créneaux prioritaires pour les éditions d'usine officielles.",
    turnkeyPreorderBtn: "Réserver un appareil prêt à l'emploi",
    turnkeyReservedNotice: "Réservation confirmée ! Votre jeton de fabrication a été enregistré.",
    turnkeyModalTitle: "Réservation matérielle prioritaire",
    turnkeyEmailLabel: "E-mail pour notification",
    turnkeySubmitBtn: "Confirmer la réservation prioritaire",
    landingTitle: "Sovereign — Serveur domestique privé",
    landingHeroHeadline: "Votre sanctuaire numérique personnel",
    landingHeroSubheadline: "Appareil serveur domestique 100 % privé et sans télémétrie. Diffusez des médias 4K, archivez des documents avec OCR, utilisez une IA locale et extrayez des disques optiques sur votre propre matériel.",
    landingCtaDashboard: "Ouvrir le tableau de bord",
    landingCtaGitHub: "Voir sur GitHub",
    landingCtaHardware: "Calculateur matériel",
    landingFeatureShowcaseTitle: "Services domestiques privés intégrés",
    landingFeatureShowcaseSubtitle: "Cinq applications préconfigurées unifiées sous un tableau de bord privé unique, fonctionnant entièrement sur votre réseau local.",
    landingFeatureVideoTitle: "Streaming vidéo 4K et transcodage",
    landingFeatureVideoDesc: "Streaming avec accélération matérielle propulsé par Jellyfin. Le mappage de tons Intel QuickSync garantit une lecture fluide.",
    landingFeatureAudioTitle: "Musique sans perte et livres audio",
    landingFeatureAudioDesc: "Navidrome offre un streaming FLAC et MP3 haute fidélité, tandis qu'Audiobookshelf synchronise la progression et les signets.",
    landingFeatureDMSTitle: "Archivage intelligent de documents",
    landingFeatureDMSDesc: "Paperless-ngx assure l'indexation OCR, la collecte automatique par e-mail et la détection intelligente des dates d'échéance.",
    landingFeatureAITitle: "IA privée locale et RAG instantané",
    landingFeatureAIDesc: "Inférence locale llama.cpp 7B/8B GGUF sur réseau isolé. Interrogez vos documents personnels sans aucune télémétrie cloud.",
    landingFeatureDiscTitle: "Extraction automatique de disques optiques",
    landingFeatureDiscDesc: "Automatic Ripping Machine (ARM) détecte CD, DVD et Blu-ray à l'insertion, extrait sans perte et éjecte automatiquement.",
    landingPrivacyTitle: "Garanties architecturales sans télémétrie",
    landingPrivacySubtitle: "Conçu pour garantir la confidentialité absolue du foyer et une haute résilience opérationnelle.",
    landingPrivacyPill1Title: "Réseau d'IA hermétique (Air-Gapped)",
    landingPrivacyPill1Desc: "Le moteur d'IA fonctionne sur un pont Docker interne isolé sans routage internet. Vos données ne quittent jamais l'appareil.",
    landingPrivacyPill2Title: "Zéro exposition du socket Docker",
    landingPrivacyPill2Desc: "Les conteneurs applicatifs n'ont aucun accès au socket Docker de l'hôte, éliminant les risques d'élévation de privilèges.",
    landingPrivacyPill3Title: "Durabilité WAL SQLite en Go pur",
    landingPrivacyPill3Desc: "SQLite en Go pur sans CGO avec Write-Ahead Logging garantit l'intégrité des bases même en cas de coupure de courant.",
    landingPrivacyPill4Title: "Sécurité de retour arrière atomique 180s",
    landingPrivacyPill4Desc: "Les mises à jour nocturnes surveillent la santé pendant 180 secondes et restaurent automatiquement l'état antérieur en cas d'anomalie.",
    landingOpenSourceTitle: "100 % Open Source & Porté par la Communauté",
    landingOpenSourceSubtitle: "Bâti sur des fondations open source éprouvées selon la méthode BMad. Inspectez le code et assemblez votre matériel en toute liberté.",
    landingNavLanding: "Présentation",
    landingHeroBadge: "100 % privé · Zéro télémétrie cloud · Qualité électroménager",
    landingStatPings: "Pings cloud",
    landingStatStorage: "Stockage local",
    landingStatSpokes: "Services intégrés",
    landingStatPower: "Consommation au repos",
    landingBadgeVideo: "⚡ Accélération matérielle QuickSync",
    landingBadgeAudio: "🎧 FLAC sans perte et livres audio multi-utilisateurs",
    landingBadgeDms: "📑 OCR automatique et extraction d'échéances",
    landingBadgeAi: "🤖 LLM local 7B/8B et RAG de documents",
    landingBadgeDisc: "💿 Numérisation autonome de disques optiques",
    landingReadGuideMedia: "Lire le guide multimédia",
    landingReadGuideAudio: "Lire le guide audio",
    landingReadGuideDms: "Lire le guide d'archivage documentaire",
    landingReadGuideAi: "Lire le guide IA locale",
    landingReadGuideDisc: "Lire le guide de numérisation de disques",
    landingFullHardwareGuides: "Guides matériels complets et feuille de route des profils",
    landingFooterBrandDesc: "Serveur domestique 100 % privé auto-hébergé",
  },
};

/**
 * Automatically detects the preferred language based on browser navigator settings,
 * Accept-Language header strings, or provided language tags, prioritizing German (DE),
 * Spanish (ES), French (FR), and defaulting to English (EN).
 */
export function detectBrowserLanguage(preferredLanguages?: readonly string[] | string): Language {
  const candidates: string[] = [];

  if (typeof preferredLanguages === "string") {
    // Parse Accept-Language header (e.g. "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")
    const parts = preferredLanguages.split(",").map((p) => {
      const [lang, qPart] = p.trim().split(";");
      const q = qPart && qPart.startsWith("q=") ? parseFloat(qPart.slice(2)) : 1.0;
      return { lang: lang.trim(), q: isNaN(q) ? 1.0 : q };
    });
    parts.sort((a, b) => b.q - a.q);
    candidates.push(...parts.map((p) => p.lang));
  } else if (Array.isArray(preferredLanguages)) {
    candidates.push(...preferredLanguages);
  } else if (typeof window !== "undefined" && typeof navigator !== "undefined") {
    if (navigator.languages && navigator.languages.length > 0) {
      candidates.push(...navigator.languages);
    } else if (navigator.language) {
      candidates.push(navigator.language);
    }
  }

  for (const cand of candidates) {
    if (!cand || typeof cand !== "string") continue;
    const clean = cand.toLowerCase().trim();
    const primary = clean.split(/[-_]/)[0];
    if (primary === "de") return "de";
    if (primary === "es") return "es";
    if (primary === "fr") return "fr";
    if (primary === "en") return "en";
  }

  return "en";
}

/**
 * Persists the selected language preference to localStorage, cookies, and document element.
 */
export function persistLanguage(lang: Language): void {
  if (typeof window !== "undefined") {
    try {
      localStorage.setItem("sovereign_lang", lang);
    } catch {
      // ignore
    }
    try {
      document.cookie = `sovereign_lang=${lang}; path=/; max-age=31536000; SameSite=Lax`;
    } catch {
      // ignore
    }
    try {
      if (typeof document !== "undefined" && document.documentElement) {
        document.documentElement.lang = lang;
      }
    } catch {
      // ignore
    }
  }
}

/**
 * Helper to normalize and match a raw language code into a supported Language.
 */
export function normalizeLanguageTag(val?: string | null): Language | null {
  if (!val || typeof val !== "string") return null;
  const primary = val.toLowerCase().trim().split(/[-_]/)[0];
  if (primary === "en" || primary === "de" || primary === "es" || primary === "fr") {
    return primary as Language;
  }
  return null;
}

/**
 * Retrieves the active language preference:
 * 1. Explicit argument / override (normalized)
 * 2. Persisted value in localStorage
 * 3. Persisted value in cookies
 * 4. Browser autodetection via navigator.languages / navigator.language (in browser context)
 * 5. Default fallback to "en"
 */
export function getInitialLanguage(savedLanguage?: string | null): Language {
  const explicit = normalizeLanguageTag(savedLanguage);
  if (explicit) {
    return explicit;
  }
  if (typeof window !== "undefined") {
    try {
      const stored = localStorage.getItem("sovereign_lang");
      const matched = normalizeLanguageTag(stored);
      if (matched) return matched;
    } catch {
      // ignore
    }
    try {
      if (typeof document !== "undefined" && document.cookie) {
        const match = document.cookie.match(/(?:^|;\s*)sovereign_lang=([^;]+)/);
        if (match && match[1]) {
          const matched = normalizeLanguageTag(match[1]);
          if (matched) return matched;
        }
      }
    } catch {
      // ignore
    }
    return detectBrowserLanguage();
  }
  return "en";
}


