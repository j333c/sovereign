export type SupportedLanguage = "en" | "de" | "es" | "fr";

export const setupTranslations: Record<
  SupportedLanguage,
  {
    title: string;
    subtitle: string;
    stepIndicator: (step: number, total: number) => string;
    next: string;
    back: string;
    complete: string;
    completing: string;
    // Step 1
    step1Title: string;
    step1Desc: string;
    langEn: string;
    langDe: string;
    langEs: string;
    langFr: string;
    // Step 2
    step2Title: string;
    step2Desc: string;
    networkStatus: string;
    networkStatusDesc: string;
    hostnameLabel: string;
    // Step 3
    step3Title: string;
    step3Desc: string;
    storageLocalOption: string;
    storageNASOption: string;
    localDiskLabel: string;
    initializeDiskBtn: string;
    diskInitialized: string;
    nasTypeLabel: string;
    nasHostLabel: string;
    nasShareLabel: string;
    nasUserLabel: string;
    nasPassLabel: string;
    testNASBtn: string;
    testingNAS: string;
    nasTestSuccess: string;
    mountNASBtn: string;
    mountingNAS: string;
    nasMounted: string;
    // Step 4
    step4Title: string;
    step4Desc: string;
    featVideo: string;
    featVideoDesc: string;
    featAudio: string;
    featAudioDesc: string;
    featDMS: string;
    featDMSDesc: string;
    featAI: string;
    featAIDesc: string;
    // Step 5
    step5Title: string;
    step5Desc: string;
    passwordLabel: string;
    confirmPasswordLabel: string;
    emailLabel: string;
    emailHint: string;
    errPasswordShort: string;
    errPasswordMismatch: string;
    errInvalidEmail: string;
    errSetupFailed: string;
  }
> = {
  en: {
    title: "Welcome to Sovereign",
    subtitle: "Set up your private home server in 5 simple steps.",
    stepIndicator: (step, total) => `Step ${step} of ${total}`,
    next: "Continue",
    back: "Back",
    complete: "Complete Setup",
    completing: "Completing Setup...",
    step1Title: "Language Selection",
    step1Desc: "Choose your preferred language for the appliance interface.",
    langEn: "English",
    langDe: "Deutsch (German)",
    langEs: "Español (Spanish)",
    langFr: "Français (French)",
    step2Title: "Network Verification",
    step2Desc: "Your server is connected to your local network and ready for household access.",
    networkStatus: "Local Network Connected",
    networkStatusDesc: "mDNS discovery broadcast active at sovereign.local",
    hostnameLabel: "Server Hostname",
    step3Title: "Storage Configuration",
    step3Desc: "Choose where your family photos, videos, and documents will be securely stored.",
    storageLocalOption: "Internal Storage Pool (Recommended)",
    storageNASOption: "Connect Network Storage (NAS)",
    localDiskLabel: "Selected Storage Drive",
    initializeDiskBtn: "Initialize Local Storage",
    diskInitialized: "Local storage initialized and mounted successfully.",
    nasTypeLabel: "Protocol",
    nasHostLabel: "NAS Server Address / IP",
    nasShareLabel: "Share Folder Name",
    nasUserLabel: "Username",
    nasPassLabel: "Password",
    testNASBtn: "Test Connection",
    testingNAS: "Testing Connection...",
    nasTestSuccess: "NAS connection verified successfully.",
    mountNASBtn: "Mount Network Storage",
    mountingNAS: "Mounting...",
    nasMounted: "Network storage share mounted successfully.",
    step4Title: "Feature Selection",
    step4Desc: "Select the private services you want to activate on your appliance.",
    featVideo: "Video Streaming",
    featVideoDesc: "Jellyfin private movie, series, and home video streaming.",
    featAudio: "Music & Audiobooks",
    featAudioDesc: "Navidrome and Audiobookshelf for family music and audio sync.",
    featDMS: "Document Archive",
    featDMSDesc: "Paperless-ngx automated OCR and searchable household records.",
    featAI: "Local AI & Private Chat",
    featAIDesc: "Private on-device AI search and assistant powered by llama.cpp.",
    step5Title: "Administrator Account",
    step5Desc: "Create an administrator password and enter an alert email for system notifications.",
    passwordLabel: "Administrator Password",
    confirmPasswordLabel: "Confirm Password",
    emailLabel: "Notification Email Address",
    emailHint: "Used exclusively for urgent hardware alerts and automated recovery notifications.",
    errPasswordShort: "Password must be at least 8 characters.",
    errPasswordMismatch: "Passwords do not match.",
    errInvalidEmail: "Please enter a valid email address.",
    errSetupFailed: "Failed to complete setup. Please verify your settings and try again.",
  },
  de: {
    title: "Willkommen bei Sovereign",
    subtitle: "Richten Sie Ihren privaten Heimserver in 5 einfachen Schritten ein.",
    stepIndicator: (step, total) => `Schritt ${step} von ${total}`,
    next: "Weiter",
    back: "Zurück",
    complete: "Einrichtung abschließen",
    completing: "Einrichtung wird abgeschlossen...",
    step1Title: "Sprachauswahl",
    step1Desc: "Wählen Sie Ihre bevorzugte Sprache für die Benutzeroberfläche.",
    langEn: "English (Englisch)",
    langDe: "Deutsch",
    langEs: "Español (Spanisch)",
    langFr: "Français (Französisch)",
    step2Title: "Netzwerkprüfung",
    step2Desc: "Ihr Server ist mit dem Heimnetzwerk verbunden und für alle Geräte erreichbar.",
    networkStatus: "Heimnetzwerk verbunden",
    networkStatusDesc: "mDNS-Erkennung aktiv unter sovereign.local",
    hostnameLabel: "Server-Hostname",
    step3Title: "Speicherkonfiguration",
    step3Desc: "Wählen Sie, wo Ihre Familienfotos, Videos und Dokumente sicher gespeichert werden.",
    storageLocalOption: "Interner Speicherpool (Empfohlen)",
    storageNASOption: "Netzwerkspeicher verbinden (NAS)",
    localDiskLabel: "Ausgewähltes Laufwerk",
    initializeDiskBtn: "Lokalen Speicher initialisieren",
    diskInitialized: "Lokaler Speicher erfolgreich initialisiert und eingebunden.",
    nasTypeLabel: "Protokoll",
    nasHostLabel: "NAS Serveradresse / IP",
    nasShareLabel: "Freigabename",
    nasUserLabel: "Benutzername",
    nasPassLabel: "Passwort",
    testNASBtn: "Verbindung testen",
    testingNAS: "Verbindung wird getestet...",
    nasTestSuccess: "NAS-Verbindung erfolgreich bestätigt.",
    mountNASBtn: "Netzwerkspeicher einbinden",
    mountingNAS: "Wird eingebunden...",
    nasMounted: "Netzwerkspeicher erfolgreich eingebunden.",
    step4Title: "Funktionsauswahl",
    step4Desc: "Wählen Sie die Dienste, die Sie auf Ihrem Heimserver aktivieren möchten.",
    featVideo: "Video-Streaming",
    featVideoDesc: "Jellyfin für Filme, Serien und Familienvideos.",
    featAudio: "Musik & Hörbücher",
    featAudioDesc: "Navidrome und Audiobookshelf für Musik und Hörbücher.",
    featDMS: "Dokumentenarchiv",
    featDMSDesc: "Paperless-ngx automatische Texterkennung für Haushaltsunterlagen.",
    featAI: "Lokale KI & Privater Chat",
    featAIDesc: "Lokale, private KI-Suche und Assistent mit llama.cpp.",
    step5Title: "Administratorkonto",
    step5Desc: "Erstellen Sie ein sicheres Kennwort und hinterlegen Sie eine Benachrichtigungs-E-Mail.",
    passwordLabel: "Administrator-Passwort",
    confirmPasswordLabel: "Passwort bestätigen",
    emailLabel: "Benachrichtigungs-E-Mail-Adresse",
    emailHint: "Wird ausschließlich für wichtige Hardware- und Wiederherstellungsalarme genutzt.",
    errPasswordShort: "Das Passwort muss mindestens 8 Zeichen lang sein.",
    errPasswordMismatch: "Die Passwörter stimmen nicht überein.",
    errInvalidEmail: "Bitte geben Sie eine gültige E-Mail-Adresse ein.",
    errSetupFailed: "Einrichtung fehlgeschlagen. Bitte überprüfen Sie Ihre Eingaben.",
  },
  es: {
    title: "Bienvenido a Sovereign",
    subtitle: "Configure su servidor doméstico privado en 5 sencillos pasos.",
    stepIndicator: (step, total) => `Paso ${step} de ${total}`,
    next: "Continuar",
    back: "Atrás",
    complete: "Completar configuración",
    completing: "Completando configuración...",
    step1Title: "Selección de idioma",
    step1Desc: "Elija su idioma preferido para la interfaz.",
    langEn: "English (Inglés)",
    langDe: "Deutsch (Alemán)",
    langEs: "Español",
    langFr: "Français (Francés)",
    step2Title: "Verificación de red",
    step2Desc: "Su servidor está conectado a la red local y listo para usarse.",
    networkStatus: "Red local conectada",
    networkStatusDesc: "Descubrimiento mDNS activo en sovereign.local",
    hostnameLabel: "Nombre del servidor",
    step3Title: "Configuración de almacenamiento",
    step3Desc: "Elija dónde se guardarán sus fotos familiares, vídeos y documentos.",
    storageLocalOption: "Almacenamiento interno (Recomendado)",
    storageNASOption: "Conectar almacenamiento de red (NAS)",
    localDiskLabel: "Disco seleccionado",
    initializeDiskBtn: "Inicializar almacenamiento local",
    diskInitialized: "Almacenamiento local inicializado y montado con éxito.",
    nasTypeLabel: "Protocolo",
    nasHostLabel: "Dirección / IP del NAS",
    nasShareLabel: "Nombre de la carpeta compartida",
    nasUserLabel: "Usuario",
    nasPassLabel: "Contraseña",
    testNASBtn: "Probar conexión",
    testingNAS: "Probando conexión...",
    nasTestSuccess: "Conexión NAS verificada con éxito.",
    mountNASBtn: "Montar almacenamiento de red",
    mountingNAS: "Montando...",
    nasMounted: "Almacenamiento de red montado con éxito.",
    step4Title: "Selección de funciones",
    step4Desc: "Elija los servicios que desea activar en su servidor.",
    featVideo: "Transmisión de vídeo",
    featVideoDesc: "Jellyfin para películas, series y vídeos familiares.",
    featAudio: "Música y audiolibros",
    featAudioDesc: "Navidrome y Audiobookshelf para sincronización de música y audio.",
    featDMS: "Archivo de documentos",
    featDMSDesc: "Paperless-ngx OCR automático para documentos del hogar.",
    featAI: "IA local y chat privado",
    featAIDesc: "Búsqueda y asistente de IA privada local impulsada por llama.cpp.",
    step5Title: "Cuenta de administrador",
    step5Desc: "Cree una contraseña segura e introduzca un correo para avisos importantes.",
    passwordLabel: "Contraseña de administrador",
    confirmPasswordLabel: "Confirmar contraseña",
    emailLabel: "Correo electrónico de notificación",
    emailHint: "Utilizado únicamente para alertas críticas del hardware y recuperación.",
    errPasswordShort: "La contraseña debe tener al menos 8 caracteres.",
    errPasswordMismatch: "Las contraseñas no coinciden.",
    errInvalidEmail: "Por favor introduzca un correo electrónico válido.",
    errSetupFailed: "Error al completar la configuración. Inténtelo de nuevo.",
  },
  fr: {
    title: "Bienvenue sur Sovereign",
    subtitle: "Configurez votre serveur privé en 5 étapes simples.",
    stepIndicator: (step, total) => `Étape ${step} sur ${total}`,
    next: "Continuer",
    back: "Retour",
    complete: "Terminer la configuration",
    completing: "Finalisation...",
    step1Title: "Sélection de la langue",
    step1Desc: "Choisissez votre langue préférée pour l'interface.",
    langEn: "English (Anglais)",
    langDe: "Deutsch (Allemand)",
    langEs: "Español (Espagnol)",
    langFr: "Français",
    step2Title: "Vérification du réseau",
    step2Desc: "Votre serveur est connecté au réseau local et accessible aux appareils.",
    networkStatus: "Réseau local connecté",
    networkStatusDesc: "Diffusion mDNS active sur sovereign.local",
    hostnameLabel: "Nom d'hôte du serveur",
    step3Title: "Configuration du stockage",
    step3Desc: "Choisissez où vos photos, vidéos et documents de famille seront stockés.",
    storageLocalOption: "Stockage interne (Recommandé)",
    storageNASOption: "Connecter un stockage réseau (NAS)",
    localDiskLabel: "Disque sélectionné",
    initializeDiskBtn: "Initialiser le stockage local",
    diskInitialized: "Stockage local initialisé et monté avec succès.",
    nasTypeLabel: "Protocole",
    nasHostLabel: "Adresse IP / Hôte du NAS",
    nasShareLabel: "Nom du partage",
    nasUserLabel: "Nom d'utilisateur",
    nasPassLabel: "Mot de passe",
    testNASBtn: "Tester la connexion",
    testingNAS: "Test en cours...",
    nasTestSuccess: "Connexion NAS vérifiée avec succès.",
    mountNASBtn: "Monter le stockage réseau",
    mountingNAS: "Montage...",
    nasMounted: "Partage réseau monté avec succès.",
    step4Title: "Sélection des fonctionnalités",
    step4Desc: "Sélectionnez les services que vous souhaitez activer.",
    featVideo: "Streaming Vidéo",
    featVideoDesc: "Jellyfin pour vos films, séries et vidéos personnelles.",
    featAudio: "Musique & Livres audio",
    featAudioDesc: "Navidrome et Audiobookshelf pour la musique et les livres audio.",
    featDMS: "Archivage de documents",
    featDMSDesc: "Paperless-ngx reconnaissance de texte et classement automatique.",
    featAI: "IA locale & Chat privé",
    featAIDesc: "Recherche et assistant IA 100% local propulsé par llama.cpp.",
    step5Title: "Compte administrateur",
    step5Desc: "Créez un mot de passe administrateur et renseignez une adresse e-mail d'alerte.",
    passwordLabel: "Mot de passe administrateur",
    confirmPasswordLabel: "Confirmer le mot de passe",
    emailLabel: "Adresse e-mail de notification",
    emailHint: "Utilisée exclusivement pour les alertes matérielles et la restauration.",
    errPasswordShort: "Le mot de passe doit comporter au moins 8 caractères.",
    errPasswordMismatch: "Les mots de passe ne correspondent pas.",
    errInvalidEmail: "Veuillez saisir une adresse e-mail valide.",
    errSetupFailed: "Échec de la configuration. Veuillez vérifier vos saisies.",
  },
};

