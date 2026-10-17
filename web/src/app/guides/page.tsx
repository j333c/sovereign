"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { AppHeader } from "@/components/AppHeader";
import { Language, translations, getInitialLanguage, persistLanguage } from "@/lib/i18n";

export type GuideTopicId = "quickstart" | "storage" | "media" | "ai" | "resilience";

export interface GuideSection {
  title: string;
  content: string[];
}

export interface GuideTopicContent {
  id: GuideTopicId;
  icon: string;
  title: string;
  summary: string;
  sections: GuideSection[];
}

export const guidesData: Record<Language, GuideTopicContent[]> = {
  en: [
    {
      id: "quickstart",
      icon: "🚀",
      title: "Quickstart & Appliance Setup",
      summary: "Connect your Sovereign home appliance to power and your router, and access the interface in under two minutes.",
      sections: [
        {
          title: "1. Hardware Connections",
          content: [
            "Plug your Sovereign appliance into AC power and connect an Ethernet cable directly from the server's gigabit LAN port to your home internet router.",
            "Press the power button. The appliance will initiate its hardened boot sequence, taking approximately 30 to 45 seconds to initialize network services."
          ]
        },
        {
          title: "2. Finding Your Appliance on the Local Network",
          content: [
            "From any phone, laptop, or tablet on your home Wi-Fi, open your browser and navigate to http://sovereign.local.",
            "If your local router does not support mDNS multicast (.local domain resolution), open your router admin page, locate 'sovereign' under DHCP clients, and enter that IP address (e.g. http://192.168.1.50) into your browser."
          ]
        },
        {
          title: "3. Guided First-Run Setup",
          content: [
            "On your first visit, the 5-step First-Run Wizard automatically launches.",
            "Select your language, verify local hostname network connectivity, initialize your local storage pool or mount an existing home NAS, select your media and AI features, and configure your master administrator password."
          ]
        }
      ]
    },
    {
      id: "storage",
      icon: "💾",
      title: "Storage & Network NAS",
      summary: "Configure local solid-state and hard drives, mount existing household NAS shares via SMB or NFS, and monitor disk health.",
      sections: [
        {
          title: "1. Internal Storage Pools",
          content: [
            "Sovereign formats internal data disks as a single resilient storage pool mounted at /data/.",
            "All databases (SQLite with Write-Ahead Logging), user media libraries, uploaded documents, and local AI weights reside under isolated directories within /data/."
          ]
        },
        {
          title: "2. Mounting External NAS Shares",
          content: [
            "Navigate to Settings > Storage & Pools to connect your existing Synology, QNAP, TrueNAS, or generic SMB/CIFS/NFS share.",
            "Use the 'Test Connection' button to verify host reachability and credentials before mounting. Sovereign safely mounts the share under /mnt/nas without modifying existing files."
          ]
        },
        {
          title: "3. Capacity Alerts & Safety Headroom",
          content: [
            "The pinned bottom telemetry bar monitors pool utilization in real time.",
            "When disk usage exceeds 85%, the indicator turns amber. At 95%, automated background downloads pause to preserve essential system operations."
          ]
        }
      ]
    },
    {
      id: "media",
      icon: "🎬",
      title: "Media Ingestion & Dropzone",
      summary: "Add movies, TV series, music albums, and audiobooks effortlessly via web drag-and-drop, network file sharing, or optical disc ripping.",
      sections: [
        {
          title: "1. Web Dropzone (/drop)",
          content: [
            "Click the 'Drop Media' button in the top navigation bar from any device.",
            "Select the destination library (Movies, TV Shows, Music, or Audiobooks) and drag media files directly into the upload area. Ingestion tracks progress and automatically indexes files into Jellyfin and Navidrome."
          ]
        },
        {
          title: "2. Local Network Share (\\\\sovereign\\drop)",
          content: [
            "On Windows, open File Explorer and type \\\\sovereign\\drop into the address bar. On macOS, open Finder, press Cmd+K, and connect to smb://sovereign.local/drop.",
            "Copy large media collections directly into the categorized folders. Background workers automatically ingest and organize new files every 60 seconds."
          ]
        },
        {
          title: "3. Automated Optical Disc Ripping",
          content: [
            "Insert any movie DVD, Blu-ray, or music audio CD into an attached optical drive.",
            "Sovereign detects disc insertion automatically, extracts tracks losslessly, queries public metadata databases for poster artwork, and automatically ejects the disc when complete."
          ]
        }
      ]
    },
    {
      id: "ai",
      icon: "🤖",
      title: "Private Local AI & Document OCR",
      summary: "100% on-device private conversational intelligence, document archival with Paperless-ngx, and dynamic GPU memory arbitration.",
      sections: [
        {
          title: "1. Strict Zero-Telemetry Boundary",
          content: [
            "The AI inference engine (llama.cpp) runs on an isolated internal Docker bridge network without public internet routing.",
            "Your prompts, questions, family documents, and conversation history never leave the physical appliance. Prompts in system logs are scrubbed and anonymized."
          ]
        },
        {
          title: "2. Document Archival & Deadline Detection",
          content: [
            "Upload utility bills, insurance policies, medical records, or tax letters to Paperless-ngx via web upload, mobile scanner, or automated IMAP email polling.",
            "The document parser automatically extracts cancellation deadlines and payment obligations, sending reassuring reminder alerts before key dates."
          ]
        },
        {
          title: "3. Smart GPU Arbitration",
          content: [
            "During the day, dedicated GPU resources prioritize interactive AI queries with high token generation speeds.",
            "When the assistant remains idle for over 5 minutes, GPU VRAM automatically surrenders to hardware-accelerated 4K video transcoding for seamless family streaming."
          ]
        }
      ]
    },
    {
      id: "resilience",
      icon: "🛡️",
      title: "Power & Rollback Safety",
      summary: "Engineered for unexpected power cuts, automated upgrade safety checks, and one-click manual restores without data loss.",
      sections: [
        {
          title: "1. Sudden Power Loss Protection",
          content: [
            "If household power is cut, SQLite databases operate in Write-Ahead Log (WAL) mode with synchronous durability.",
            "Upon power restoration, Sovereign automatically boots into diagnostic verification mode, verifies database integrity, mounts data pools safely, and returns to normal operation within seconds."
          ]
        },
        {
          title: "2. Automated 180-Second Health Verification",
          content: [
            "Whenever an application or container upgrade occurs, Sovereign captures a pre-upgrade state snapshot and monitors service health for 180 seconds.",
            "If any container crashes, loops, or fails health probes, the system automatically triggers an atomic rollback to the previous working version, displaying a reassuring notice."
          ]
        },
        {
          title: "3. One-Click Manual Restores & Data Preservation",
          content: [
            "Administrators can restore any service to its previous working version at any time via Settings > Services.",
            "Your files, media, and document database under /data/ are completely preserved and never modified or deleted during upgrades or rollbacks."
          ]
        }
      ]
    }
  ],
  de: [
    {
      id: "quickstart",
      icon: "🚀",
      title: "Schnellstart & Einrichtung",
      summary: "Verbinden Sie Ihren Sovereign-Heimserver mit Strom und Router und greifen Sie in unter zwei Minuten darauf zu.",
      sections: [
        {
          title: "1. Hardware anschließen",
          content: [
            "Stecken Sie das Stromkabel ein und verbinden Sie den Gigabit-Netzwerkanschluss Ihres Sovereign-Servers per Netzwerkkabel direkt mit Ihrem Router.",
            "Drücken Sie den Netzschalter. Das System startet den abgesicherten Boot-Vorgang und initialisiert Netzwerkdienste in ca. 30 bis 45 Sekunden."
          ]
        },
        {
          title: "2. Server im Heimnetzwerk aufrufen",
          content: [
            "Öffnen Sie von einem beliebigen Smartphone, Laptop oder Tablet in Ihrem WLAN den Browser und rufen Sie http://sovereign.local auf.",
            "Unterstützt Ihr Router kein mDNS (.local), suchen Sie in der Weboberfläche Ihres Routers nach 'sovereign' und rufen Sie die angezeigte IP-Adresse auf (z. B. http://192.168.1.50)."
          ]
        },
        {
          title: "3. Geführte Ersteinrichtung",
          content: [
            "Beim ersten Aufruf startet automatisch der 5-stufige Einrichtungsassistent.",
            "Wählen Sie Ihre Sprache, prüfen Sie die Netzwerkanbindung, initialisieren Sie den internen Speicher oder binden Sie ein NAS ein, wählen Sie gewünschte Dienste und vergeben Sie das Administrator-Passwort."
          ]
        }
      ]
    },
    {
      id: "storage",
      icon: "💾",
      title: "Speicher & Netzwerk-NAS",
      summary: "Konfigurieren Sie interne Festplatten, binden Sie bestehende NAS-Freigaben (SMB/NFS) ein und überwachen Sie die Speicherkapazität.",
      sections: [
        {
          title: "1. Interner Speicherpool",
          content: [
            "Sovereign formatiert interne Datenlaufwerke als robusten Speicherpool unter /data/.",
            "Alle Datenbanken (SQLite im Write-Ahead-Log-Modus), Mediendateien, Dokumente und KI-Modelle liegen sicher und isoliert unter /data/."
          ]
        },
        {
          title: "2. Externes NAS einbinden",
          content: [
            "Öffnen Sie Einstellungen > Speicher & Pools, um Ihr bestehendes Synology-, QNAP- oder TrueNAS-System per SMB oder NFS einzubinden.",
            "Nutzen Sie die Schaltfläche 'Verbindung testen', um Zugangsdaten vorab zu prüfen. Sovereign bindet das Netzlaufwerk unter /mnt/nas ein, ohne bestehende Dateien zu verändern."
          ]
        },
        {
          title: "3. Kapazitätswarnungen & Sicherheitsreserve",
          content: [
            "Die fixierte Statusleiste am unteren Bildschirmrand zeigt die Speichernutzung in Echtzeit an.",
            "Ab 85 % Belegung färbt sich die Anzeige bernsteinfarben. Bei 95 % pausieren automatische Hintergrund-Downloads, um den reibungslosen Betrieb zu sichern."
          ]
        }
      ]
    },
    {
      id: "media",
      icon: "🎬",
      title: "Medien & Dropzone",
      summary: "Fügen Sie Filme, Serien, Musikalben und Hörbücher bequem per Web-Upload, Netzwerkfreigabe oder Disc-Laufwerk hinzu.",
      sections: [
        {
          title: "1. Web-Dropzone (/drop)",
          content: [
            "Klicken Sie in der oberen Navigation auf 'Medien ablegen'.",
            "Wählen Sie die Zielkategorie (Filme, Serien, Musik, Hörbücher) und ziehen Sie Dateien per Drag & Drop hinein. Der Server verarbeitet und indiziert die Medien automatisch in Jellyfin und Navidrome."
          ]
        },
        {
          title: "2. Lokale Netzwerkfreigabe (\\\\sovereign\\drop)",
          content: [
            "Unter Windows geben Sie im Windows Explorer \\\\sovereign\\drop ein. Auf dem Mac drücken Sie im Finder Cmd+K und verbinden sich mit smb://sovereign.local/drop.",
            "Kopieren Sie große Mediensammlungen direkt in die Ordner. Hintergrundprozesse prüfen alle 60 Sekunden auf neue Dateien und ordnen sie ein."
          ]
        },
        {
          title: "3. Automatisches Einlesen von CDs & DVDs",
          content: [
            "Legen Sie eine Film-DVD, Blu-ray oder Musik-CD in ein angeschlossenes optisches Laufwerk ein.",
            "Sovereign erkennt die Disc automatisch, liest Titel verlustfrei ein, ergänzt Coverbilder und Titelinformationen und wirft die Disc nach Abschluss selbstständig aus."
          ]
        }
      ]
    },
    {
      id: "ai",
      icon: "🤖",
      title: "Private lokale KI & Dokumentenverwaltung",
      summary: "100 % lokale Intelligenz ohne Cloud-Übertragung, rechtssichere Dokumentenablage mit Paperless-ngx und intelligente Hardware-Aufteilung.",
      sections: [
        {
          title: "1. Strikter Datenschutz ohne Telemetrie",
          content: [
            "Die KI-Inferenz-Engine (llama.cpp) läuft in einem abgeschirmten internen Docker-Netzwerk ohne Internetzugriff.",
            "Ihre Anfragen, Familienunterlagen und Chatverläufe verlassen niemals das Gerät. Prompts in Systemprotokollen werden automatisch geschwärzt."
          ]
        },
        {
          title: "2. Dokumentenarchivierung & Fristenerkennung",
          content: [
            "Laden Sie Rechnungen, Verträge oder Briefe per Web-Upload, mobiler Scan-App oder automatischer IMAP-E-Mail-Abfrage hoch.",
            "Die Texterkennung (OCR) identifiziert Kündigungsfristen und Zahlungstermine automatisch und erinnert Sie rechtzeitig vor Fälligkeit."
          ]
        },
        {
          title: "3. Intelligente Hardware-Aufteilung (GPU Sharing)",
          content: [
            "Tagsüber steht der Grafikspeicher (VRAM) mit höchster Priorität für schnelle KI-Antworten bereit.",
            "Bei Inaktivität von über 5 Minuten wird der Grafikspeicher automatisch für flüssige 4K-Videotranskodierung im Wohnzimmer freigegeben."
          ]
        }
      ]
    },
    {
      id: "resilience",
      icon: "🛡️",
      title: "Ausfallsicherheit & Rollback",
      summary: "Geschützt gegen plötzlichen Stromausfall, automatische Integritätsprüfung und Ein-Klick-Wiederherstellung ohne Datenverlust.",
      sections: [
        {
          title: "1. Schutz vor Stromausfällen",
          content: [
            "Bei Stromausfall schützt der Write-Ahead-Log (WAL) von SQLite vor Dateibeschädigung.",
            "Nach der Rückkehr des Stroms führt Sovereign automatische Systemtests durch, bindet Speicher sicher ein und ist innerhalb von Sekunden wieder betriebsbereit."
          ]
        },
        {
          title: "2. Automatische 180-Sekunden-Überprüfung",
          content: [
            "Vor jedem Update erstellt Sovereign einen Sicherheits-Schnappschuss und überwacht die Dienste 180 Sekunden lang.",
            "Tritt ein Fehler auf, wird das Update automatisch und ohne Datenverlust auf die funktionierende Version zurückgesetzt."
          ]
        },
        {
          title: "3. Manuelle Ein-Klick-Wiederherstellung",
          content: [
            "In den Einstellungen können Sie jeden Dienst jederzeit manuell auf die vorherige Version zurücksetzen.",
            "Ihre gespeicherten Dokumente, Medien und Daten unter /data/ bleiben bei Updates und Rollbacks stets unberührt und sicher erhalten."
          ]
        }
      ]
    }
  ],
  es: [
    {
      id: "quickstart",
      icon: "🚀",
      title: "Inicio rápido y configuración",
      summary: "Conecte su electrodoméstico Sovereign a la corriente y al rúter para comenzar en menos de dos minutos.",
      sections: [
        {
          title: "1. Conexiones físicas",
          content: [
            "Conecte el cable de alimentación y conecte un cable de red Ethernet desde el puerto gigabit de Sovereign a su rúter doméstico.",
            "Presione el botón de encendido. El sistema iniciará su arranque seguro en aproximadamente 30 a 45 segundos."
          ]
        },
        {
          title: "2. Acceder al servidor en su red local",
          content: [
            "Desde cualquier teléfono, tableta o portátil en su red Wi-Fi, abra el navegador y entre en http://sovereign.local.",
            "Si su rúter no admite mDNS (.local), consulte la lista de clientes en la interfaz del rúter y use la dirección IP asignada (por ej. http://192.168.1.50)."
          ]
        },
        {
          title: "3. Asistente inicial paso a paso",
          content: [
            "En su primera visita se iniciará el asistente de 5 pasos.",
            "Seleccione el idioma, verifique la conexión de red, configure el almacenamiento interno o NAS, elija las funciones deseadas y defina su contraseña de administrador."
          ]
        }
      ]
    },
    {
      id: "storage",
      icon: "💾",
      title: "Almacenamiento y NAS en red",
      summary: "Configure discos duros internos, monte recursos NAS domésticos por SMB/NFS y supervise la capacidad disponible.",
      sections: [
        {
          title: "1. Grupo de almacenamiento local",
          content: [
            "Sovereign formatea los discos locales en un grupo resistente montado en /data/.",
            "Las bases de datos con registro por adelantado (WAL), las bibliotecas multimedia y los documentos residen de forma segura y aislada en /data/."
          ]
        },
        {
          title: "2. Montar almacenamiento NAS externo",
          content: [
            "En Configuración > Almacenamiento y grupos puede conectar su servidor NAS Synology, QNAP o TrueNAS mediante SMB o NFS.",
            "Pruebe las credenciales con el botón 'Probar conexión'. Sovereign monta el recurso en /mnt/nas sin alterar sus archivos existentes."
          ]
        },
        {
          title: "3. Alertas de capacidad y margen de seguridad",
          content: [
            "La barra inferior muestra el uso del disco en tiempo real.",
            "Al superar el 85 % el indicador pasa a ámbar. Al llegar al 95 % las descargas se pausan automáticamente para proteger el funcionamiento del sistema."
          ]
        }
      ]
    },
    {
      id: "media",
      icon: "🎬",
      title: "Medios y zona de entrega",
      summary: "Añada películas, series, música y audiolibros fácilmente mediante la web, carpetas de red compartidas o discos ópticos.",
      sections: [
        {
          title: "1. Zona de entrega web (/drop)",
          content: [
            "Haga clic en 'Depositar medios' en la barra superior.",
            "Elija la biblioteca de destino (Películas, Series, Música o Audiolibros) y arrastre sus archivos. Sovereign los procesa e indexa automáticamente en Jellyfin y Navidrome."
          ]
        },
        {
          title: "2. Carpeta compartida en red local (\\\\sovereign\\drop)",
          content: [
            "En Windows abra el Explorador de archivos e introduzca \\\\sovereign\\drop. En Mac abra el Finder, pulse Cmd+K y conéctese a smb://sovereign.local/drop.",
            "Copie colecciones completas a las carpetas. Los procesos en segundo plano indexan los nuevos archivos automáticamente cada minuto."
          ]
        },
        {
          title: "3. Extracción automática de discos ópticos",
          content: [
            "Inserte un DVD, Blu-ray o CD de música en una unidad óptica conectada.",
            "Sovereign detecta el disco, extrae el contenido sin pérdida de calidad, descarga carátulas y expulsa el disco al terminar."
          ]
        }
      ]
    },
    {
      id: "ai",
      icon: "🤖",
      title: "IA local y privada",
      summary: "Inteligencia artificial 100 % en el dispositivo sin telemetría, archivo de documentos con Paperless-ngx y uso compartido inteligente de GPU.",
      sections: [
        {
          title: "1. Privacidad absoluta sin telemetría",
          content: [
            "El motor de inferencia local (llama.cpp) funciona en una red interna aislada sin acceso a Internet.",
            "Sus preguntas, datos familiares y documentos nunca salen del electrodoméstico. Los registros del sistema se anonimizan automáticamente."
          ]
        },
        {
          title: "2. Archivo documental y detección de vencimientos",
          content: [
            "Suba facturas, pólizas o contratos mediante la web, escáner móvil o importación automática por correo IMAP.",
            "El sistema extrae fechas límite de pago o rescisión y envía alertas preventivas para evitar recargos."
          ]
        },
        {
          title: "3. Uso compartido de GPU (Hardware Sharing)",
          content: [
            "Durante el día, la GPU da prioridad al asistente de IA con respuestas rápidas e interactivas.",
            "Tras 5 minutos de inactividad, la memoria gráfica se cede automáticamente a la transcodificación de vídeo 4K en el televisor."
          ]
        }
      ]
    },
    {
      id: "resilience",
      icon: "🛡️",
      title: "Resiliencia eléctrica y restauración",
      summary: "Protegido contra cortes de luz repentinos, verificación automática de actualizaciones y restauración con un clic sin pérdida de datos.",
      sections: [
        {
          title: "1. Protección ante cortes de suministro",
          content: [
            "En caso de apagón repentino, SQLite en modo WAL previene la corrupción de datos.",
            "Al restablecerse la corriente, Sovereign realiza una comprobación diagnóstica y vuelve a operar con total normalidad en segundos."
          ]
        },
        {
          title: "2. Verificación de salud de 180 segundos",
          content: [
            "Antes de cada actualización se genera una instantánea del estado anterior y se supervisan los servicios durante 180 segundos.",
            "Si un contenedor falla, el sistema regresa automáticamente a la versión previa que funcionaba correctamente."
          ]
        },
        {
          title: "3. Restauración manual en un solo clic",
          content: [
            "En Configuración > Servicios puede revertir cualquier servicio a su versión anterior cuando lo desee.",
            "Sus archivos, documentos y fotos en /data/ se conservan intactos y nunca se alteran durante una actualización o reversión."
          ]
        }
      ]
    }
  ],
  fr: [
    {
      id: "quickstart",
      icon: "🚀",
      title: "Démarrage rapide & Configuration",
      summary: "Branchez votre serveur Sovereign et accédez à l'interface d'administration en moins de deux minutes.",
      sections: [
        {
          title: "1. Raccordements matériels",
          content: [
            "Branchez le cordon d'alimentation et reliez le port Ethernet gigabit de votre appareil Sovereign directement à votre box Internet.",
            "Allumez l'appareil. Le démarrage sécurisé initialise les services réseau en 30 à 45 secondes environ."
          ]
        },
        {
          title: "2. Trouver l'appareil sur votre réseau local",
          content: [
            "Depuis n'importe quel smartphone, tablette ou ordinateur sur votre Wi-Fi, ouvrez votre navigateur sur http://sovereign.local.",
            "Si votre box ne gère pas le mDNS (.local), repérez 'sovereign' dans la liste des baux DHCP de votre box et ouvrez l'adresse IP affichée (ex. http://192.168.1.50)."
          ]
        },
        {
          title: "3. Assistant de premier démarrage",
          content: [
            "Lors de votre première visite, l'assistant guidé en 5 étapes démarre automatiquement.",
            "Choisissez la langue, validez la connexion réseau, initialisez le stockage local ou connectez un NAS, sélectionnez vos services et définissez votre mot de passe administrateur."
          ]
        }
      ]
    },
    {
      id: "storage",
      icon: "💾",
      title: "Stockage & NAS réseau",
      summary: "Gérez les disques durs internes, montez vos partages NAS existants (SMB/NFS) et surveillez l'espace disponible.",
      sections: [
        {
          title: "1. Pool de stockage local",
          content: [
            "Sovereign formate les disques locaux au sein d'un pool résilient monté sous /data/.",
            "Les bases de données (SQLite en mode WAL), les bibliothèques multimédias et les documents sont stockés de manière isolée et sécurisée."
          ]
        },
        {
          title: "2. Montage de stockages NAS externes",
          content: [
            "Rendez-vous dans Paramètres > Stockage & Pools pour relier votre NAS Synology, QNAP ou TrueNAS en SMB ou NFS.",
            "Testez la connexion avant validation. Sovereign monte le partage sous /mnt/nas sans jamais modifier vos fichiers d'origine."
          ]
        },
        {
          title: "3. Alertes de capacité et marge de sécurité",
          content: [
            "La barre de télémétrie en bas d'écran affiche l'occupation du disque en temps réel.",
            "À partir de 85 % d'occupation, l'indicateur devient orange. À 95 %, les téléchargements automatiques s'interrompent pour préserver le système."
          ]
        }
      ]
    },
    {
      id: "media",
      icon: "🎬",
      title: "Médias & Zone de dépôt",
      summary: "Ajoutez facilement vos films, séries, musiques et livres audio via l'interface web, le partage réseau ou vos disques optiques.",
      sections: [
        {
          title: "1. Zone de dépôt web (/drop)",
          content: [
            "Cliquez sur 'Déposer des médias' dans la barre de navigation supérieure.",
            "Sélectionnez la catégorie (Films, Séries, Musique ou Livres audio) et glissez-déposez vos fichiers. Sovereign les intègre et les indexe automatiquement dans Jellyfin et Navidrome."
          ]
        },
        {
          title: "2. Dossier partagé sur le réseau local (\\\\sovereign\\drop)",
          content: [
            "Sous Windows, tapez \\\\sovereign\\drop dans l'Explorateur de fichiers. Sur Mac, ouvrez le Finder, appuyez sur Cmd+K et connectez-vous à smb://sovereign.local/drop.",
            "Copiez vos fichiers directement dans les dossiers prévus. Le serveur traite et range les nouveaux médias toutes les minutes."
          ]
        },
        {
          title: "3. Numérisation automatique de disques optiques",
          content: [
            "Insérez un DVD, Blu-ray ou CD audio dans un lecteur optique connecté à l'appareil.",
            "Sovereign détecte le disque, extrait les pistes sans perte, télécharge les jaquettes et métadonnées, puis éjecte le disque automatiquement."
          ]
        }
      ]
    },
    {
      id: "ai",
      icon: "🤖",
      title: "IA locale et privée",
      summary: "Assistant intelligent 100 % sur l'appareil sans télémétrie, gestion électronique de documents Paperless-ngx et partage intelligent de GPU.",
      sections: [
        {
          title: "1. Zéro télémétrie et respect de la vie privée",
          content: [
            "Le moteur d'inférence (llama.cpp) fonctionne sur un réseau Docker interne isolé de tout accès Internet extérieur.",
            "Vos messages, documents de famille et historiques ne quittent jamais votre domicile. Les données privées dans les journaux sont masquées automatiquement."
          ]
        },
        {
          title: "2. Archivage de documents et détection d'échéances",
          content: [
            "Numérisez vos factures, contrats ou courriers via le web, l'application mobile ou la relève automatique d'e-mails IMAP.",
            "L'analyseur repère automatiquement les dates de résiliation et échéances de paiement pour vous alerter à temps."
          ]
        },
        {
          title: "3. Partage intelligent du GPU (Smart Arbitration)",
          content: [
            "En journée, la mémoire graphique (VRAM) est allouée en priorité à l'IA pour des réponses immédiates.",
            "Après 5 minutes d'inactivité, la mémoire est libérée pour le transcodage matériel des flux vidéo 4K sur vos téléviseurs."
          ]
        }
      ]
    },
    {
      id: "resilience",
      icon: "🛡️",
      title: "Résilience électrique & Restauration",
      summary: "Immunité contre les coupures de courant, contrôle automatique de santé des mises à jour et restauration en un clic sans perte de données.",
      sections: [
        {
          title: "1. Protection contre les pannes de courant",
          content: [
            "En cas de coupure brutale, le mode WAL de SQLite garantit l'intégrité absolue des bases de données.",
            "Au retour du courant, Sovereign lance automatiquement une vérification de diagnostic et redevient opérationnel en quelques secondes."
          ]
        },
        {
          title: "2. Vérification automatique de santé sur 180 secondes",
          content: [
            "Avant chaque mise à jour, Sovereign crée un instantané de sécurité et surveille les conteneurs pendant 180 secondes.",
            "En cas d'instabilité, le système revient immédiatement et de manière autonome à la version antérieure fonctionnelle."
          ]
        },
        {
          title: "3. Restauration manuelle en un clic",
          content: [
            "Dans Paramètres > Services, vous pouvez restaurer un service sur sa version précédente d'un simple clic.",
            "Tous vos fichiers, médias et documents dans /data/ restent totalement préservés lors de chaque mise à jour ou retour arrière."
          ]
        }
      ]
    }
  ]
};

export interface GuidesPageProps {
  initialLanguage?: Language;
  initialTopic?: GuideTopicId;
}

export default function GuidesPage({ initialLanguage, initialTopic = "quickstart" }: GuidesPageProps = {}) {
  const [lang, setLang] = useState<Language>(() => {
    return getInitialLanguage(initialLanguage);
  });
  const t = translations[lang];

  const handleLanguageChange = (newLang: Language) => {
    setLang(newLang);
    persistLanguage(newLang);
  };

  const [activeTopicId, setActiveTopicId] = useState<GuideTopicId>(initialTopic);
  const topics = guidesData[lang] || guidesData.en;
  const currentTopic = topics.find((top) => top.id === activeTopicId) || topics[0];
  const navRef = useRef<HTMLElement>(null);

  // Synchronize hash in URL (#storage, #media, #ai, #resilience, #quickstart)
  useEffect(() => {
    if (typeof window !== "undefined") {
      const handleHashChange = () => {
        const hash = window.location.hash.replace("#", "") as GuideTopicId;
        if (["quickstart", "storage", "media", "ai", "resilience"].includes(hash)) {
          setActiveTopicId(hash);
        }
      };

      if (!initialTopic && window.location.hash) {
        handleHashChange();
      }

      window.addEventListener("hashchange", handleHashChange);
      return () => window.removeEventListener("hashchange", handleHashChange);
    }
  }, [initialTopic]);

  const selectTopic = (id: GuideTopicId) => {
    setActiveTopicId(id);
    if (typeof window !== "undefined") {
      window.location.hash = id;
    }
  };

  // Keyboard navigation for sidebar topic buttons
  const handleNavKeyDown = (e: React.KeyboardEvent<HTMLElement>) => {
    const topicIds: GuideTopicId[] = ["quickstart", "storage", "media", "ai", "resilience"];
    const currentIndex = topicIds.indexOf(activeTopicId);
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIndex = (currentIndex + 1) % topicIds.length;
      selectTopic(topicIds[nextIndex]);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIndex = (currentIndex - 1 + topicIds.length) % topicIds.length;
      selectTopic(topicIds[prevIndex]);
    }
  };

  return (
    <div className="flex min-h-screen flex-col bg-[#F8F9FA] text-[#0F172A] font-sans">
      {/* 64px Header */}
      <AppHeader
        currentLanguage={lang}
        onLanguageChange={handleLanguageChange}
        aggregateStatus="operational"
      />

      {/* Main Guides Layout */}
      <div className="mx-auto flex w-full max-w-[1200px] flex-1 flex-col gap-6 px-4 py-8 sm:px-6 sm:py-10 md:grid md:grid-cols-[280px_1fr] md:gap-8">
        {/* Sidebar Topics Navigation */}
        <aside>
          <div className="mb-4">
            <Link
              href="/"
              className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-[#E5E7EB] bg-white px-3 py-2 text-xs sm:text-sm font-medium text-[#0F172A] shadow-xs hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              aria-label={t.backToDashboard}
            >
              <span aria-hidden="true">←</span>
              <span>{t.backToDashboard}</span>
            </Link>
          </div>

          <div className="mb-3 px-1">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-[#64748B]">
              {t.guidesTitle}
            </h2>
          </div>

          <nav
            ref={navRef}
            onKeyDown={handleNavKeyDown}
            className="flex flex-col gap-1.5"
            aria-label="Guide topics"
            role="tablist"
          >
            {topics.map((top) => {
              const isSelected = top.id === activeTopicId;
              return (
                <button
                  key={top.id}
                  type="button"
                  role="tab"
                  id={`tab-${top.id}`}
                  aria-selected={isSelected}
                  aria-controls={`panel-${top.id}`}
                  onClick={() => selectTopic(top.id)}
                  className={`flex min-h-[44px] items-center gap-3 rounded-lg px-3.5 py-2.5 text-xs sm:text-sm font-medium text-left transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${
                    isSelected
                      ? "bg-white text-[#0F172A] font-semibold border border-[#E5E7EB] shadow-xs"
                      : "text-[#475569] hover:bg-[#F1F5F9] hover:text-[#0F172A]"
                  }`}
                  data-testid={`guide-tab-${top.id}`}
                >
                  <span className="text-lg" aria-hidden="true">{top.icon}</span>
                  <span className="truncate">{top.title}</span>
                </button>
              );
            })}
          </nav>
        </aside>

        {/* Content Viewer Pane */}
        <main
          id={`panel-${currentTopic.id}`}
          role="tabpanel"
          aria-labelledby={`tab-${currentTopic.id}`}
          className="flex flex-col gap-6"
          data-testid="guide-content-panel"
        >
          <section className="rounded-xl border border-[#E5E7EB] bg-white p-6 sm:p-8 shadow-xs">
            {/* Header */}
            <div className="flex items-start gap-4 pb-6 border-b border-[#F1F5F9]">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-blue-50 text-2xl border border-blue-100" aria-hidden="true">
                {currentTopic.icon}
              </div>
              <div className="flex-1">
                <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[#0F172A]">
                  {currentTopic.title}
                </h1>
                <p className="mt-1 text-sm text-[#64748B] leading-relaxed">
                  {currentTopic.summary}
                </p>
              </div>
            </div>

            {/* Guide Sections */}
            <div className="mt-8 space-y-8 divide-y divide-[#F1F5F9]">
              {currentTopic.sections.map((sec, idx) => (
                <div key={idx} className={idx > 0 ? "pt-8" : ""}>
                  <h3 className="text-base font-semibold text-[#0F172A] mb-3">
                    {sec.title}
                  </h3>
                  <div className="space-y-3">
                    {sec.content.map((p, pIdx) => (
                      <p key={pIdx} className="text-sm text-[#475569] leading-relaxed">
                        {p}
                      </p>
                    ))}
                  </div>
                </div>
              ))}
            </div>

            {/* Bottom Actions */}
            <div className="mt-10 flex flex-wrap items-center justify-between gap-4 border-t border-[#F1F5F9] pt-6">
              <Link
                href="/settings"
                className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-[#E5E7EB] bg-[#F8F9FA] px-4 py-2 text-xs sm:text-sm font-medium text-[#0F172A] hover:bg-[#F1F5F9] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              >
                <span>⚙️</span>
                <span>{t.settings}</span>
              </Link>

              <Link
                href="/drop"
                className="inline-flex min-h-[44px] items-center gap-2 rounded-lg bg-blue-600 px-5 py-2.5 text-xs sm:text-sm font-semibold text-white shadow-xs hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 transition-colors"
              >
                <span>📥</span>
                <span>{t.dropMedia}</span>
              </Link>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}
