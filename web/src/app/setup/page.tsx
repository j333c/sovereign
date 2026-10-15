"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  apiClient as defaultApiClient,
  SovereignApiClient,
  DiskInfo,
  ProblemDetailsError,
} from "@/lib/api";

export type SupportedLanguage = "en" | "de" | "es" | "fr";

export interface SetupPageProps {
  apiClientOverride?: SovereignApiClient;
  routerOverride?: { replace: (url: string) => void; push?: (url: string) => void };
  initialStep?: number;
  initialLanguage?: SupportedLanguage;
}

const translations: Record<
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

export function SetupScreen(props: SetupPageProps) {
  const router = props.routerOverride;
  const client = props.apiClientOverride ?? defaultApiClient;

  const [step, setStep] = useState<number>(props.initialStep ?? 1);
  const [language, setLanguage] = useState<SupportedLanguage>(() => {
    if (props.initialLanguage) return props.initialLanguage;
    if (typeof navigator !== "undefined" && navigator.language) {
      const code = navigator.language.slice(0, 2).toLowerCase();
      if (code === "de" || code === "es" || code === "fr") {
        return code as SupportedLanguage;
      }
    }
    return "en";
  });
  const t = translations[language] || translations.en;

  // If already configured, redirect to operational dashboard
  useEffect(() => {
    let isMounted = true;
    client
      .getSetupStatus()
      .then((res) => {
        if (!isMounted) return;
        if (res.is_configured && router) {
          if (typeof router.replace === "function") {
            router.replace("/");
          } else if (typeof router.push === "function") {
            router.push("/");
          }
        }
      })
      .catch(() => {});

    return () => {
      isMounted = false;
    };
  }, [client, router]);

  // Step 2 state
  const [hostname, setHostname] = useState("sovereign.local");

  // Step 3 state
  const [storageMode, setStorageMode] = useState<"local" | "nas">("local");
  const [disks, setDisks] = useState<DiskInfo[]>([]);
  const [selectedDisk, setSelectedDisk] = useState<string>("/dev/nvme0n1");
  const [diskInitSuccess, setDiskInitSuccess] = useState(false);
  const [diskInitLoading, setDiskInitLoading] = useState(false);

  const [nasType, setNasType] = useState<"smb" | "nfs">("smb");
  const [nasHost, setNasHost] = useState("");
  const [nasShare, setNasShare] = useState("");
  const [nasUser, setNasUser] = useState("");
  const [nasPass, setNasPass] = useState("");
  const [nasTestStatus, setNasTestStatus] = useState<string | null>(null);
  const [nasTestError, setNasTestError] = useState<string | null>(null);
  const [nasTesting, setNasTesting] = useState(false);
  const [nasMountSuccess, setNasMountSuccess] = useState(false);
  const [nasMounting, setNasMounting] = useState(false);

  // Step 4 state
  const [features, setFeatures] = useState<{
    video: boolean;
    audio: boolean;
    dms: boolean;
    ai: boolean;
  }>({
    video: true,
    audio: true,
    dms: true,
    ai: true,
  });

  // Step 5 state
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [email, setEmail] = useState("");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Fetch block devices on step 3 load
  useEffect(() => {
    if (step === 3 && storageMode === "local") {
      let isMounted = true;
      client
        .getDisks()
        .then((res) => {
          if (!isMounted) return;
          if (res.disks && res.disks.length > 0) {
            setDisks(res.disks);
            setSelectedDisk(res.disks[0].path);
          }
        })
        .catch(() => {
          // Keep default disk fallback
        });
      return () => {
        isMounted = false;
      };
    }
  }, [step, storageMode, client]);

  // Handle local disk initialize
  const handleInitializeDisk = async () => {
    setDiskInitLoading(true);
    setErrorMsg(null);
    try {
      await client.initializeStorage({ device: selectedDisk });
      setDiskInitSuccess(true);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setErrorMsg(err.detail);
      } else {
        setErrorMsg("Storage initialization failed. Please check device.");
      }
    } finally {
      setDiskInitLoading(false);
    }
  };

  // Handle NAS test
  const handleTestNAS = async () => {
    setNasTesting(true);
    setNasTestError(null);
    setNasTestStatus(null);
    try {
      const res = await client.testNAS({
        type: nasType,
        host: nasHost,
        share: nasShare,
        username: nasUser,
        password: nasPass,
      });
      if (res.success) {
        setNasTestStatus(t.nasTestSuccess);
      } else {
        setNasTestError(res.message);
      }
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setNasTestError(err.detail);
      } else {
        setNasTestError("NAS connection failed. Check host and credentials.");
      }
    } finally {
      setNasTesting(false);
    }
  };

  // Handle NAS mount
  const handleMountNAS = async () => {
    setNasMounting(true);
    setNasTestError(null);
    try {
      await client.mountNAS({
        type: nasType,
        host: nasHost,
        share: nasShare,
        username: nasUser,
        password: nasPass,
      });
      setNasMountSuccess(true);
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setNasTestError(err.detail);
      } else {
        setNasTestError("Failed to mount network storage.");
      }
    } finally {
      setNasMounting(false);
    }
  };

  // Handle Step 5 completion
  const handleCompleteSetup = async () => {
    setErrorMsg(null);

    if (password.length < 8) {
      setErrorMsg(t.errPasswordShort);
      return;
    }

    if (password !== confirmPassword) {
      setErrorMsg(t.errPasswordMismatch);
      return;
    }

    const emailValid = email.includes("@") && email.includes(".");
    if (!emailValid) {
      setErrorMsg(t.errInvalidEmail);
      return;
    }

    setIsSubmitting(true);

    try {
      const selectedFeatureList: string[] = [];
      if (features.video) selectedFeatureList.push("video");
      if (features.audio) selectedFeatureList.push("audio");
      if (features.dms) selectedFeatureList.push("dms");
      if (features.ai) selectedFeatureList.push("ai");

      await client.completeSetup({
        username: "admin",
        password,
        email,
        language,
        features: selectedFeatureList,
        hostname,
      });

      if (router) {
        if (typeof router.replace === "function") {
          router.replace("/");
        } else if (typeof router.push === "function") {
          router.push("/");
        }
      }
    } catch (err: unknown) {
      if (err instanceof ProblemDetailsError) {
        setErrorMsg(err.detail);
      } else {
        setErrorMsg(t.errSetupFailed);
      }
      setIsSubmitting(false);
    }
  };

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-4 sm:p-8 bg-[var(--canvas)]">
      <div className="w-full max-w-xl rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 sm:p-8 shadow-sm">
        {/* Wizard Header */}
        <div className="pb-6 border-b border-[var(--border)]">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold tracking-wider uppercase text-[var(--accent-blue)]">
              {t.stepIndicator(step, 5)}
            </span>
            <span className="text-xs text-[var(--text-muted)] font-mono">
              {hostname}
            </span>
          </div>
          {/* Progress Bar */}
          <div className="mt-3 h-1.5 w-full rounded-full bg-[var(--border-subtle)] overflow-hidden">
            <div
              className="h-full bg-[var(--accent-blue)] transition-all duration-300"
              style={{ width: `${(step / 5) * 100}%` }}
              role="progressbar"
              aria-valuenow={step}
              aria-valuemin={1}
              aria-valuemax={5}
            />
          </div>
          <h1 className="mt-4 text-xl font-semibold tracking-tight text-[var(--text-primary)]">
            {step === 1 && t.step1Title}
            {step === 2 && t.step2Title}
            {step === 3 && t.step3Title}
            {step === 4 && t.step4Title}
            {step === 5 && t.step5Title}
          </h1>
          <p className="mt-1 text-sm text-[var(--text-muted)] leading-relaxed">
            {step === 1 && t.step1Desc}
            {step === 2 && t.step2Desc}
            {step === 3 && t.step3Desc}
            {step === 4 && t.step4Desc}
            {step === 5 && t.step5Desc}
          </p>
        </div>

        {/* Wizard Body Steps */}
        <div className="py-6">
          {/* STEP 1: Language Selection */}
          {step === 1 && (
            <div className="space-y-3" role="radiogroup" aria-label="Language selection">
              {(
                [
                  { code: "en", label: t.langEn, flag: "🇬🇧" },
                  { code: "de", label: t.langDe, flag: "🇩🇪" },
                  { code: "es", label: t.langEs, flag: "🇪🇸" },
                  { code: "fr", label: t.langFr, flag: "🇫🇷" },
                ] as const
              ).map((item) => (
                <button
                  key={item.code}
                  type="button"
                  onClick={() => setLanguage(item.code)}
                  className={`w-full flex items-center justify-between p-4 rounded-lg border text-left transition-all ${
                    language === item.code
                      ? "border-[var(--accent-blue)] bg-blue-50/30 text-[var(--text-primary)] font-medium ring-1 ring-[var(--accent-blue)]"
                      : "border-[var(--border)] hover:bg-[var(--border-subtle)] text-[var(--text-primary)]"
                  }`}
                >
                  <span className="flex items-center gap-3">
                    <span className="text-xl">{item.flag}</span>
                    <span className="text-sm">{item.label}</span>
                  </span>
                  {language === item.code && (
                    <span className="h-2 w-2 rounded-full bg-[var(--accent-blue)]" />
                  )}
                </button>
              ))}
            </div>
          )}

          {/* STEP 2: Network Verification */}
          {step === 2 && (
            <div className="space-y-6">
              <div className="flex items-start gap-4 p-4 rounded-lg bg-emerald-50 border border-emerald-200">
                <span className="mt-0.5 text-emerald-600 text-lg">●</span>
                <div>
                  <h3 className="text-sm font-semibold text-emerald-950">
                    {t.networkStatus}
                  </h3>
                  <p className="mt-0.5 text-xs text-emerald-800">
                    {t.networkStatusDesc}
                  </p>
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">
                  {t.hostnameLabel}
                </label>
                <input
                  type="text"
                  value={hostname}
                  onChange={(e) => setHostname(e.target.value)}
                  className="w-full rounded-lg border border-[var(--border)] p-3 text-sm font-mono text-[var(--text-primary)] bg-[var(--surface)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)]"
                />
              </div>
            </div>
          )}

          {/* STEP 3: Storage Configuration */}
          {step === 3 && (
            <div className="space-y-5">
              <div className="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={() => setStorageMode("local")}
                  className={`p-3 rounded-lg border text-left text-xs font-medium transition-all ${
                    storageMode === "local"
                      ? "border-[var(--accent-blue)] bg-blue-50/40 text-[var(--text-primary)] ring-1 ring-[var(--accent-blue)]"
                      : "border-[var(--border)] hover:bg-[var(--border-subtle)] text-[var(--text-muted)]"
                  }`}
                >
                  {t.storageLocalOption}
                </button>
                <button
                  type="button"
                  onClick={() => setStorageMode("nas")}
                  className={`p-3 rounded-lg border text-left text-xs font-medium transition-all ${
                    storageMode === "nas"
                      ? "border-[var(--accent-blue)] bg-blue-50/40 text-[var(--text-primary)] ring-1 ring-[var(--accent-blue)]"
                      : "border-[var(--border)] hover:bg-[var(--border-subtle)] text-[var(--text-muted)]"
                  }`}
                >
                  {t.storageNASOption}
                </button>
              </div>

              {storageMode === "local" && (
                <div className="space-y-4 pt-2">
                  <div>
                    <label className="block text-xs font-medium text-[var(--text-muted)] mb-1.5">
                      {t.localDiskLabel}
                    </label>
                    <select
                      value={selectedDisk}
                      onChange={(e) => {
                        setSelectedDisk(e.target.value);
                        setDiskInitSuccess(false);
                      }}
                      className="w-full rounded-lg border border-[var(--border)] p-3 text-sm text-[var(--text-primary)] bg-[var(--surface)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)]"
                    >
                      {disks.length > 0 ? (
                        disks.map((d) => (
                          <option key={d.path} value={d.path}>
                            {d.path} - {d.model || d.name} ({(d.size_bytes / 1e9).toFixed(1)} GB)
                          </option>
                        ))
                      ) : (
                        <option value="/dev/nvme0n1">/dev/nvme0n1 - Internal NVMe Pool (1.0 TB)</option>
                      )}
                    </select>
                  </div>

                  <button
                    type="button"
                    onClick={handleInitializeDisk}
                    disabled={diskInitLoading || diskInitSuccess}
                    className="w-full rounded-lg bg-[var(--action-secondary)] border border-[var(--border)] hover:bg-[var(--border-subtle)] text-xs font-medium text-[var(--text-primary)] py-2.5 transition-colors disabled:opacity-50"
                  >
                    {diskInitLoading ? "Formatting..." : diskInitSuccess ? "✓ " + t.diskInitialized : t.initializeDiskBtn}
                  </button>

                  {diskInitSuccess && (
                    <p className="text-xs text-emerald-700 font-medium">
                      ✓ {t.diskInitialized}
                    </p>
                  )}
                </div>
              )}

              {storageMode === "nas" && (
                <div className="space-y-3 pt-2">
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                        {t.nasTypeLabel}
                      </label>
                      <select
                        value={nasType}
                        onChange={(e) => setNasType(e.target.value as "smb" | "nfs")}
                        className="w-full rounded-lg border border-[var(--border)] p-2 text-xs text-[var(--text-primary)] bg-[var(--surface)]"
                      >
                        <option value="smb">SMB / CIFS (Windows/Samba)</option>
                        <option value="nfs">NFS (Network File System)</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                        {t.nasHostLabel}
                      </label>
                      <input
                        type="text"
                        placeholder="192.168.1.100"
                        value={nasHost}
                        onChange={(e) => setNasHost(e.target.value)}
                        className="w-full rounded-lg border border-[var(--border)] p-2 text-xs text-[var(--text-primary)] bg-[var(--surface)]"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                      {t.nasShareLabel}
                    </label>
                    <input
                      type="text"
                      placeholder="media"
                      value={nasShare}
                      onChange={(e) => setNasShare(e.target.value)}
                      className="w-full rounded-lg border border-[var(--border)] p-2 text-xs text-[var(--text-primary)] bg-[var(--surface)]"
                    />
                  </div>

                  {nasType === "smb" && (
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                          {t.nasUserLabel}
                        </label>
                        <input
                          type="text"
                          value={nasUser}
                          onChange={(e) => setNasUser(e.target.value)}
                          className="w-full rounded-lg border border-[var(--border)] p-2 text-xs text-[var(--text-primary)] bg-[var(--surface)]"
                        />
                      </div>
                      <div>
                        <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                          {t.nasPassLabel}
                        </label>
                        <input
                          type="password"
                          value={nasPass}
                          onChange={(e) => setNasPass(e.target.value)}
                          className="w-full rounded-lg border border-[var(--border)] p-2 text-xs text-[var(--text-primary)] bg-[var(--surface)]"
                        />
                      </div>
                    </div>
                  )}

                  <div className="flex gap-2 pt-2">
                    <button
                      type="button"
                      onClick={handleTestNAS}
                      disabled={nasTesting || !nasHost}
                      className="flex-1 rounded-lg bg-[var(--action-secondary)] border border-[var(--border)] py-2 text-xs font-medium text-[var(--text-primary)] hover:bg-[var(--border-subtle)] disabled:opacity-50"
                    >
                      {nasTesting ? t.testingNAS : t.testNASBtn}
                    </button>
                    <button
                      type="button"
                      onClick={handleMountNAS}
                      disabled={nasMounting || !nasHost || nasMountSuccess}
                      className="flex-1 rounded-lg bg-[var(--action-primary)] text-white py-2 text-xs font-medium hover:bg-slate-800 disabled:opacity-50"
                    >
                      {nasMounting ? t.mountingNAS : nasMountSuccess ? "✓ Mounted" : t.mountNASBtn}
                    </button>
                  </div>

                  {nasTestStatus && (
                    <p className="text-xs text-emerald-700 font-medium">✓ {nasTestStatus}</p>
                  )}
                  {nasTestError && (
                    <p className="text-xs text-red-600 font-medium">▲ {nasTestError}</p>
                  )}
                  {nasMountSuccess && (
                    <p className="text-xs text-emerald-700 font-medium">✓ {t.nasMounted}</p>
                  )}
                </div>
              )}
            </div>
          )}

          {/* STEP 4: Feature Checklist */}
          {step === 4 && (
            <div className="space-y-3">
              {[
                {
                  id: "video" as const,
                  title: t.featVideo,
                  desc: t.featVideoDesc,
                  checked: features.video,
                },
                {
                  id: "audio" as const,
                  title: t.featAudio,
                  desc: t.featAudioDesc,
                  checked: features.audio,
                },
                {
                  id: "dms" as const,
                  title: t.featDMS,
                  desc: t.featDMSDesc,
                  checked: features.dms,
                },
                {
                  id: "ai" as const,
                  title: t.featAI,
                  desc: t.featAIDesc,
                  checked: features.ai,
                },
              ].map((feat) => (
                <label
                  key={feat.id}
                  className={`flex items-start gap-3 p-4 rounded-lg border cursor-pointer transition-all ${
                    feat.checked
                      ? "border-[var(--accent-blue)] bg-blue-50/20"
                      : "border-[var(--border)] hover:bg-[var(--border-subtle)]"
                  }`}
                >
                  <input
                    type="checkbox"
                    checked={feat.checked}
                    onChange={(e) =>
                      setFeatures((prev) => ({
                        ...prev,
                        [feat.id]: e.target.checked,
                      }))
                    }
                    className="mt-1 h-4 w-4 rounded border-gray-300 text-[var(--accent-blue)] focus:ring-[var(--focus-ring)]"
                  />
                  <div className="flex-1">
                    <span className="text-sm font-medium text-[var(--text-primary)]">
                      {feat.title}
                    </span>
                    <p className="text-xs text-[var(--text-muted)] mt-0.5">
                      {feat.desc}
                    </p>
                  </div>
                </label>
              ))}
            </div>
          )}

          {/* STEP 5: Admin Account */}
          {step === 5 && (
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                  {t.passwordLabel}
                </label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••••••"
                  className="w-full rounded-lg border border-[var(--border)] p-3 text-sm text-[var(--text-primary)] bg-[var(--surface)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)]"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                  {t.confirmPasswordLabel}
                </label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="••••••••••••"
                  className="w-full rounded-lg border border-[var(--border)] p-3 text-sm text-[var(--text-primary)] bg-[var(--surface)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)]"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                  {t.emailLabel}
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="alex@family.net"
                  className="w-full rounded-lg border border-[var(--border)] p-3 text-sm text-[var(--text-primary)] bg-[var(--surface)] focus:outline-none focus:ring-2 focus:ring-[var(--focus-ring)]"
                />
                <p className="mt-1 text-xs text-[var(--text-muted)]">
                  {t.emailHint}
                </p>
              </div>
            </div>
          )}

          {errorMsg && (
            <div className="mt-4 p-3 rounded-lg bg-red-50 border border-red-200 text-xs text-red-700 font-medium">
              ▲ {errorMsg}
            </div>
          )}
        </div>

        {/* Wizard Footer Controls */}
        <div className="pt-4 border-t border-[var(--border)] flex items-center justify-between">
          {step > 1 ? (
            <button
              type="button"
              onClick={() => {
                setErrorMsg(null);
                setStep((s) => s - 1);
              }}
              disabled={isSubmitting}
              className="px-4 py-2 text-xs font-medium text-[var(--text-primary)] hover:bg-[var(--border-subtle)] rounded-lg border border-[var(--border)] transition-colors"
            >
              {t.back}
            </button>
          ) : (
            <div />
          )}

          {step < 5 ? (
            <button
              type="button"
              onClick={() => {
                setErrorMsg(null);
                if (step === 2) {
                  const trimmed = hostname.trim();
                  const validHostname = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/.test(trimmed);
                  if (!validHostname) {
                    setErrorMsg("Please enter a valid hostname (e.g. sovereign.local).");
                    return;
                  }
                }
                if (step === 3) {
                  if (storageMode === "local" && !diskInitSuccess) {
                    setErrorMsg("Please initialize your local storage before continuing.");
                    return;
                  }
                  if (storageMode === "nas" && !nasMountSuccess) {
                    setErrorMsg("Please mount your network storage before continuing.");
                    return;
                  }
                }
                setStep((s) => s + 1);
              }}
              className="px-5 py-2 text-xs font-medium text-white bg-[var(--action-primary)] hover:bg-slate-800 rounded-lg transition-colors shadow-sm"
            >
              {t.next}
            </button>
          ) : (
            <button
              type="button"
              onClick={handleCompleteSetup}
              disabled={isSubmitting}
              className="px-5 py-2 text-xs font-medium text-white bg-[var(--action-primary)] hover:bg-slate-800 rounded-lg transition-colors shadow-sm disabled:opacity-50"
            >
              {isSubmitting ? t.completing : t.complete}
            </button>
          )}
        </div>
      </div>
    </main>
  );
}

export default function SetupPage() {
  const router = useRouter();
  return <SetupScreen routerOverride={router} />;
}
