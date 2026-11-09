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

## 6. Configuration SELinux pour Red Hat et Fedora
- **Contextes de volumes pour conteneurs (`container_file_t`) :** Sur Fedora, Red Hat Enterprise Linux, CentOS Stream, AlmaLinux et Rocky Linux, SELinux est activé en mode Enforcing par défaut. Les dossiers locaux montés dans les conteneurs Docker (tels que `/data/` et `/opt/sovereign`) doivent obligatoirement porter le contexte `container_file_t` pour autoriser les accès en lecture et écriture.
- **Prise en charge automatique par l'installateur :** Le script d'installation de Sovereign détecte automatiquement les environnements SELinux, configure des politiques permanentes via `semanage fcontext` (ou des étiquettes `chcon`) et maintient l'intégrité des contextes lors des mises à jour.
- **Application immédiate du contexte :** Pour appliquer manuellement l'étiquette à tous les répertoires de données :
  ```bash
  sudo chcon -Rt container_file_t /data /opt/sovereign
  ```
- **Politique permanente résistante aux redémarrages :** Pour préserver cette configuration lors des mises à jour ou réétiquetages du système, enregistrez la règle à l'aide de `semanage` (`policycoreutils-python-utils`) :
  ```bash
  sudo semanage fcontext -a -t container_file_t "/data(/.*)?"
  sudo semanage fcontext -a -t container_file_t "/opt/sovereign(/.*)?"
  sudo restorecon -Rv /data /opt/sovereign
  ```
- **Drapeaux de volumes Docker Compose (`:z` vs `:Z`) :** Lors de l'ajout de conteneurs accédant à `/data`, utilisez impérativement le drapeau `:z` (volume partagé) plutôt que `:Z`. Comme plusieurs applications Sovereign (Jellyfin, Navidrome, Paperless) partagent l'accès à `/data`, le drapeau `:Z` réserve l'accès exclusif à un conteneur unique et bloque tous les autres services.
- **Stockage NAS externe (NFS / SMB) :** Les systèmes de fichiers réseau ne gèrent pas les attributs étendus standards de SELinux (`security.selinux`), entraînant l'échec de `chcon`. Pour monter un partage réseau sous `/data` ou `/mnt/nas` :
  - Spécifiez l'option de contexte explicite dans `/etc/fstab` ou via la commande mount :
    ```bash
    mount -t nfs -o context="system_u:object_r:container_file_t:s0" nas.local:/volume1/media /mnt/nas
    ```
  - Activez les booléens SELinux correspondants si nécessaire :
    ```bash
    sudo setsebool -P virt_use_nfs 1
    sudo setsebool -P virt_use_samba 1
    ```
- **Diagnostic des blocages de permissions :** Si un service signale une erreur `Permission denied` lors de l'accès à `/data`, inspectez les alertes AVC du journal d'audit SELinux :
  ```bash
  sudo ausearch -m avc -ts recent
  ```
  ou consultez le journal système et les journaux d'audit :
  ```bash
  sudo journalctl -g AVC
  sudo grep AVC /var/log/audit/audit.log
  ```

