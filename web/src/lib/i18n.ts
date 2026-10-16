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
  },
};
