# Guides de l'appareil Sovereign (Français)

## 1. Démarrage rapide & Configuration
- **Installateur en une seule ligne :** Sur n'importe quelle machine Linux propre, exécutez `curl -fsSL https://raw.githubusercontent.com/j333c/sovereign/main/install.sh | sudo bash` pour installer Docker automatiquement, déployer `/opt/sovereign` et démarrer le serveur.
- **Branchements matériels :** Raccordez le câble d'alimentation et branchez le câble réseau Ethernet directement à votre box Internet. Allumez le serveur.
- **Accès sur le réseau local :** Rendez-vous sur `http://sovereign.local` depuis n'importe quel appareil sur votre Wi-Fi. Si votre box ne gère pas le mDNS, cherchez l'IP de `sovereign` dans la liste DHCP de votre box (ex. `http://192.168.1.50`).
- **Assistant de premier lancement :** Suivez les 5 étapes pour choisir votre langue, tester la connexion réseau, initialiser le stockage, activer vos modules et choisir le mot de passe administrateur.

## 2. Stockage & NAS réseau
- **Pool de stockage interne :** Les disques durs locaux sont regroupés au sein d'un pool résilient sous `/data/`. Le mode WAL de SQLite garantit une durabilité maximale.
- **Connexion d'un NAS externe :** Rendez-vous dans `Paramètres > Stockage & Pools` pour monter un partage Synology, QNAP ou TrueNAS en SMB ou NFS. Validez les identifiants avec le bouton "Tester la connexion".
- **Surveillance de la capacité :** La barre de télémétrie en bas d'écran indique l'espace utilisé en temps réel. L'indicateur devient orange à 85 %, puis interrompt les téléchargements à 95 % par mesure de sécurité.

## 3. Médias & Zone de dépôt
- **Zone de dépôt web :** Rendez-vous sur `/drop` pour glisser-déposer des films, séries, morceaux musicaux ou livres audio. Ils sont indexés automatiquement dans Jellyfin et Navidrome.
- **Partage réseau local :** Connectez-vous à `\\sovereign\drop` sous Windows ou à `smb://sovereign.local/drop` sous macOS pour transférer de gros volumes de fichiers.
- **Numérisation automatique de disques :** Insérez un DVD, Blu-ray ou CD audio dans un lecteur optique branché. Les pistes sont extraites sans compression avec jaquettes, puis le disque est éjecté.

## 4. IA locale et privée
- **Zéro télémétrie :** Le moteur d'inférence (`llama.cpp`) est confiné sur un réseau interne isolé d'Internet. Vos requêtes et documents confidentiels ne quittent jamais votre foyer.
- **Gestion documentaire et OCR :** Importez vos factures, avis fiscaux et contrats dans Paperless-ngx. L'analyseur détecte automatiquement les dates d'échéance et de résiliation pour vous alerter.
- **Partage dynamique du GPU :** La mémoire vidéo (VRAM) est réservée en journée pour des réponses IA instantanées, puis bascule sur le transcodage 4K fluide après 5 minutes sans requête.

## 5. Résilience électrique & Restauration
- **Résistance aux pannes de courant :** La journalisation WAL préserve l'intégrité des données même en cas de coupure brutale du secteur. Le diagnostic s'exécute automatiquement au redémarrage.
- **Retour arrière automatique (180s) :** Lors de chaque mise à jour, la santé des services est contrôlée pendant 180 secondes. En cas d'anomalie, la version stable précédente est restaurée automatiquement.
- **Restauration manuelle en un clic :** Dans `Paramètres > Services`, revenez à une version antérieure quand vous le souhaitez. Vos données et médias dans `/data/` restent toujours intacts.
