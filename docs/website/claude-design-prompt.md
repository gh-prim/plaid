# PLAID website: Claude Design prompt

Paste everything below the line into Claude Design, with the PLAID design system selected.

---

Design the marketing website for PLAID: 8 pages, in English and French, with the copy below (English first, then the French copy for the same pages). Use the PLAID design system for everything (tokens, type, components, logo). Do not invent colors, fonts or components; if something is missing, build it from existing tokens and flag it.

## What PLAID is
A self-hosted, open-source (MIT) media server, an open alternative to Plex. It indexes the movies and shows on your own disks and streams them to every screen in the house. Its signature: watching together, in the same room or miles apart, is built in rather than bolted on. The project is in early development and the site must say so honestly.

## Audience
Self-hosters and home-lab people (NAS, Raspberry Pi, Docker) who install it, and the less technical friends and family they invite. Convince the first, reassure the second.

## Art direction (from the design system)
- The interface is the dark room around the screen: warm near-black neutrals, quiet type (Instrument Sans for UI and body, Instrument Serif for display headlines).
- A single warm color, the amber thread, used only for focus, progress and the one primary action per screen. It comes from the yellow over-check of the Gordon tartan.
- The tartan appears woven thread by thread in the logo and in the played part of timelines, and nowhere else. No tartan wallpaper, no tartan backgrounds.
- The posters bring the color. Use placeholder poster artwork (invented titles, no real films, no studio logos).
- Dark by default, plus a light mode. Generous radii, soft shadows, slow and gentle motion. WCAG AA minimum, a strong visible focus ring.
- Tone: warm, cozy, a little playful, never salesy, never apologetic.

## Global constraints
- Responsive: desktop (1440), tablet (768), phone (375), no horizontal scroll.
- All copy is translatable (English source, French first translation, about 20% longer): no text baked into images, leave room in buttons and headings.
- Code blocks are copyable, with a "Copied" state.
- Status labels everywhere use the same component: Available, In progress, Next, Planned.

## Shared layout

**Header:** PLAID logo. Nav: Features · Plaid Party · Install · Roadmap · FAQ. Right side: language switch (EN / FR), theme toggle, "GitHub" ghost button, "Install" primary button (amber). On phone: logo + menu button opening a full-screen sheet.

**Footer:**
- Tagline: "Plaid & chill. Your cinema under the blanket."
- Columns: Product (Features, Plaid Party, Install, Roadmap) · Project (Open source, FAQ, GitHub, Releases) · Docs (Install guide, API reference)
- Bottom line: "MIT licensed. Made by two people and a couch." · language switch · theme toggle

**Early-development banner** (thin bar under the header, dismissible): "PLAID is in early development. The server installs and runs today; libraries and playback are next. Follow along on GitHub →"

---

## Page 1: Home (`/`)

**Hero**
- Eyebrow: Self-hosted · Open source · MIT
- H1: Your movies and shows, on your own server.
- H1 second line (serif, softer): Watch alone, or together.
- Lead: PLAID indexes the films and series on your disks and streams them to every screen in the house. No central account, no subscription, no one watching what you watch.
- Primary CTA: Install PLAID · Secondary CTA: View on GitHub
- Visual: the PLAID web app home screen (rows of placeholder posters, a "Continue watching" row with amber progress) inside a TV frame, in a dark living room.

**Section: Plaid Party teaser**
- H2: Movie night, even miles apart.
- Body: Start a Plaid Party, send a link, and everyone lands on the same frame. The server keeps the clock, so nobody counts "3, 2, 1, play" over the phone again.
- Visual: four devices (Apple TV, laptop, Android phone, iPad), all paused at 1:12:48 on the same film, each with a small participant avatar.
- Link: Discover Plaid Party →

**Section: Three pillars** (3 cards)
1. **Yours, really.** Your files stay on your disks. Your server, your accounts, your rules. No phoning home.
2. **Every screen.** Web today. iPhone, iPad, Apple TV, Android and Android TV next, all from one server.
3. **Made for sharing.** Invite friends by link, choose exactly which libraries they see, revoke in one click.

**Section: Runs on what you already have**
- H2: One binary. A Raspberry Pi is enough.
- Body: PLAID is a single Go binary with SQLite and FFmpeg. Install the Debian package, run the script, or start the Docker container. It starts on boot and stays out of your way.
- Logos row (monochrome, as labels): Ubuntu · Debian · Docker · Raspberry Pi · amd64 · arm64
- CTA: See install options →

**Section: Final CTA**
- H2: Grab a blanket. Bring your own films.
- Buttons: Install PLAID · Star on GitHub

---

## Page 2: Features (`/features`)

**Hero**
- H1: Everything you expect from a media server.
- Lead: The basics done properly, on your hardware. Then a few things nobody else does.

**Feature groups** (each group: H2 + short intro + grid of cards with icon, title, one sentence, status label)

*Your libraries*
- **Folder scanning** (Available): Point PLAID at your movie and show folders. It finds new files on its own.
- **Smart matching** (In progress): `Movie (2019).mkv` and `S01E02` become proper titles, seasons and episodes.
- **Posters and synopses** (In progress): Artwork, cast and summaries from TMDB and fanart.tv, in each library's language.
- **Search and collections** (Planned): Find anything in a keystroke, group films the way you like.

*Playback*
- **Direct Play** (Next): When your device can play the file as is, PLAID sends it untouched. Full quality, no CPU spent.
- **Remux and transcode** (Next): When it can't, PLAID adapts it on the fly with FFmpeg.
- **Hardware acceleration** (Next): VAAPI, Intel Quick Sync, NVENC and VideoToolbox, so a small box can still transcode.
- **Subtitles** (Planned): Text subtitles everywhere, image subtitles burned in when a device can't draw them.
- **Resume anywhere** (Planned): Stop on the TV, pick up on your phone. "Continue watching" follows you.

*People*
- **Accounts and profiles** (Available): An admin, local accounts, and a profile per person.
- **Library sharing** (Planned): See the Sharing block below.
- **Plaid Party** (Planned): Watch together, in sync. See the Plaid Party page.

**Block: Sharing, without a middleman**
- H2: Share a library, not your whole server.
- Body: Send a friend a single-use invitation link. They create an account on your server and only ever see what you shared: one library, or one collection.
- Bullet list: Per-library or per-collection sharing · Maximum quality per friend · Downloads on or off · Number of simultaneous streams · Instant revocation, active streams included
- Visual: an invitation sheet UI with those toggles.

**Block: Comparison table**
- H2: How PLAID compares.
- Columns: PLAID · Typical hosted media servers
- Rows: Central account required (No / Usually yes) · Works with a plain public URL or tunnel (Yes / Varies) · Watch-together built in (Yes, planned / Add-on or none) · Source code (MIT / Closed or partial) · Telemetry (None / Varies)
- Footnote: "Planned" means designed, not shipped yet. See the roadmap.

---

## Page 3: Plaid Party (`/party`)

**Hero**
- Eyebrow: The flagship feature · Planned
- H1: Same film. Same second. Different couches.
- Lead: A Plaid Party keeps everyone on the same frame, wherever they are and whatever they watch on.
- CTA: Follow the progress on GitHub

**How it works** (3 numbered steps with illustrations)
1. **Start a party.** Pick a film, tap "Plaid Party". You get a link and a short code.
2. **Send the link.** Friends join from a TV, a laptop or a phone. You see who's here and who's still loading.
3. **Press play once.** Play, pause and seek reach everyone together, within half a second.

**Section: Each person gets their own stream**
- H2: 4K on the TV. 720p on the train.
- Body: Everyone watches the same moment, but each device gets the stream that suits it: Direct Play on the living room TV, a lighter transcode on a phone on mobile data. Nobody waits for the slowest connection.
- Visual: a room diagram with four participants, each tagged with device and quality.

**Section: Sync that you never notice**
- H2: No jumps, no "wait, where are you?"
- Body: The server holds the clock. When someone drifts, PLAID nudges their speed slightly rather than jumping. After a buffer, they slide back in on their own.

**Section: The social bits** (cards)
- **Reactions**: Emoji that float over the film, for everyone.
- **Chat**: A side panel that never covers the picture.
- **Bathroom break**: Anyone can ask for a pause. The host decides, or everyone can, your call.
- **Who controls**: "Host only" or "Everyone". Switch any time.

**Visual: the Party room screen** (full-width mock): player with the amber played timeline, participant avatars with presence states (watching, loading, paused), a reaction burst, the chat panel, a "Sam asks for a pause" toast with Accept / Keep watching.

**Closing line**: Video and voice chat stay out of v1 on purpose. You probably already have a call open.

---

## Page 4: Install (`/install`)

**Hero**
- H1: Up and running in five minutes.
- Lead: Pick the way that suits your machine. All of them end the same way: open `http://<your-server>:32500`.
- Note (info callout): PLAID is in early development. Until the first public release, see the install guide to build the packages yourself.

**Tabs: Ubuntu / Debian (script) · Debian package · Docker**

*Script*
- Text: Installs a system service that starts on boot. No Docker needed.
- Code: `curl -fsSL https://raw.githubusercontent.com/gh-prim/plaid/main/packaging/install.sh | sudo bash`

*Debian package*
- Steps: 1. Download `plaid_<version>_amd64.deb` (or `_arm64.deb`) and `checksums.txt` from the releases page. 2. Check it (optional): `sha256sum -c --ignore-missing checksums.txt` 3. Install: `sudo apt install ./plaid_<version>_amd64.deb`
- "What you get" table: `/usr/bin/plaid` the server · `/etc/plaid/plaid.env` configuration, kept on upgrades · `/var/lib/plaid` your data, never deleted automatically · `plaid.service` enabled and started

*Docker*
- Code: `git clone https://github.com/gh-prim/plaid.git && cd plaid` then `docker compose up -d`
- Text: Mount your media folders read-only in `docker-compose.yml`.

**Section: Requirements** (compact table)
- Systems: Ubuntu 24.04 LTS and 26.04 LTS, Debian 12 and later
- Architectures: amd64 (most PCs), arm64 (Raspberry Pi 4 and 5 with a 64-bit OS, ARM servers)
- Also installed: FFmpeg, from your distribution

**Section: Next steps** (3 cards)
- **Create the admin account**: the first visit walks you through it.
- **Add your folders**: movies and shows, each with its own language.
- **Read the full guide**: configuration, upgrades, folder permissions. → docs/install.md on GitHub

**Empty-state illustration with copy**: "Nothing to watch yet? Add a folder and come back to snuggle in."

---

## Page 5: Roadmap (`/roadmap`)

**Hero**
- H1: Where we are, honestly.
- Lead: PLAID is built in the open, one feature at a time. Each one starts as a written intent, then a spec, then a plan, all on GitHub.

**Timeline** (vertical, grouped by phase, each item with status label)
- Phase 0, Foundations: API contract, CI, Docker, Ubuntu/Debian package (Done) · Design system (In progress)
- Phase 1, Server: Accounts and authentication (Done) · Libraries and scanning (In progress) · Metadata and artwork (In progress) · Playback: Direct Play and transcoding (Next)
- Phase 2, Web app: Onboarding and administration (Planned) · Browsing and player (Planned)
- Phase 3, Together: Library sharing (Planned) · Plaid Party (Planned)
- Phase 4, Apple: iPhone, iPad, Apple TV (Planned)
- Phase 5, Android: Android and Android TV (Planned)
- Later: offline downloads, Plaid Hub, music, photos, intro and credits skipping

**Callout**: Want to see how a feature is decided? Every intent, spec and plan lives in `sdlc/` on GitHub. → Browse the specs

---

## Page 6: Open source (`/open-source`)

**Hero**
- H1: Open, readable, and yours to fork.
- Lead: PLAID is MIT licensed. The whole thing: server, web app, and the apps to come.

**Section: How we build** (3 cards)
- **No code without a plan.** Every feature goes intent → spec → plan → code, each step reviewed.
- **One API contract.** The API is described once in OpenAPI; the server and every client are generated from it.
- **Every word translatable.** English first, French next, and room for your language.

**Section: The stack** (label list)
Go · SQLite · FFmpeg · React · SwiftUI · Kotlin and Jetpack Compose

**Section: Contribute**
- Body: Found a bug, have an idea, speak a language we don't? Open an issue or a discussion. Translations are a great first contribution.
- Buttons: Open an issue · Read the contributing guide

---

## Page 7: FAQ (`/faq`)

Accordion, grouped.

*The basics*
- **Is PLAID free?** Yes. It's open source under the MIT license, with no paid tier.
- **Can I use it today?** The server installs and runs, with accounts and library scanning. Playback and the apps are next. Follow the roadmap for dates as they firm up.
- **Is it a Plex or Jellyfin fork?** No. PLAID is written from scratch, with its own API and its own apps.

*Your data*
- **Does PLAID need an online account?** No. Accounts live on your server. Nothing is required from us.
- **Does it send anything to you?** No telemetry. The server only reaches out to fetch posters and synopses from TMDB and fanart.tv.
- **Where are my files?** Exactly where they were. PLAID reads your folders; it never moves or changes your media.

*Hardware*
- **Will it run on a Raspberry Pi?** Yes, a Pi 4 or 5 with a 64-bit OS. For heavy transcoding, a machine with hardware acceleration helps.
- **Do I need Docker?** No. The Debian package installs a regular system service.

*Watching with friends*
- **How do friends reach my server from outside?** Through a public address, a reverse proxy, or a tunnel like Tailscale or Cloudflare Tunnel. The install guide covers it.
- **Do my friends need to install anything?** A browser is enough. The apps will come later.

**Bottom card**: Still wondering? Ask on GitHub Discussions.

---

## Page 8: 404

- Illustration: an empty couch with a folded blanket, the TV off.
- H1: This page went to make popcorn.
- Body: It's not here, and it might not come back.
- Button: Back to the couch (→ home)

---

# French copy (`fr`)

Same pages, same structure, same order as the English copy above. Use it for the French mockups and to check that every layout holds with longer text. The French voice uses "tu" (informal), never "vous". Code, paths, URLs and product names (PLAID, Plaid Party, Direct Play) stay as they are.

## Shared layout (FR)

**Header:** Fonctionnalités · Plaid Party · Installer · Roadmap · FAQ. Buttons: « GitHub » · « Installer ». Mobile menu button: « Menu ».

**Footer:**
- Tagline : « Plaid & chill. Ton cinéma sous la couverture. »
- Columns: Produit (Fonctionnalités, Plaid Party, Installer, Roadmap) · Projet (Open source, FAQ, GitHub, Versions) · Docs (Guide d'installation, Référence de l'API)
- Bottom line : « Sous licence MIT. Fait par deux personnes et un canapé. »

**Early-development banner:** « PLAID est en tout début de développement. Le serveur s'installe et tourne dès aujourd'hui ; les bibliothèques et la lecture arrivent ensuite. Suis l'avancée sur GitHub → »

---

## Page 1 : Accueil (`/fr/`)

**Hero**
- Eyebrow : Auto-hébergé · Open source · MIT
- H1 : Tes films et tes séries, sur ton propre serveur.
- H1 second line : Seul, ou ensemble.
- Lead : PLAID indexe les films et séries de tes disques et les diffuse sur tous les écrans de la maison. Pas de compte central, pas d'abonnement, personne pour regarder ce que tu regardes.
- CTA : Installer PLAID · Voir sur GitHub

**Section : aperçu Plaid Party**
- H2 : La soirée film, même à des kilomètres.
- Body : Lance une Plaid Party, envoie un lien, et tout le monde tombe sur la même image. Le serveur tient l'horloge : fini le « 3, 2, 1, play » au téléphone.
- Link : Découvrir Plaid Party →

**Section : trois piliers**
1. **Vraiment à toi.** Tes fichiers restent sur tes disques. Ton serveur, tes comptes, tes règles. Rien ne part ailleurs.
2. **Tous les écrans.** Le web aujourd'hui. iPhone, iPad, Apple TV, Android et Android TV ensuite, depuis un seul serveur.
3. **Fait pour partager.** Invite tes amis par lien, choisis exactement les bibliothèques qu'ils voient, retire l'accès en un clic.

**Section : tourne sur ce que tu as déjà**
- H2 : Un seul binaire. Un Raspberry Pi suffit.
- Body : PLAID, c'est un unique binaire Go avec SQLite et FFmpeg. Installe le paquet Debian, lance le script ou démarre le conteneur Docker. Il se lance au démarrage et se fait oublier.
- CTA : Voir les options d'installation →

**Section : appel final**
- H2 : Attrape un plaid. Apporte tes films.
- Buttons : Installer PLAID · Mettre une étoile sur GitHub

---

## Page 2 : Fonctionnalités (`/fr/features`)

**Hero**
- H1 : Tout ce qu'on attend d'un serveur multimédia.
- Lead : Les bases bien faites, sur ton matériel. Et quelques trucs que personne d'autre ne fait.

**Status labels :** Disponible · En cours · Prochainement · Prévu

*Tes bibliothèques*
- **Scan des dossiers** (Disponible) : Indique à PLAID tes dossiers de films et de séries. Il repère tout seul les nouveaux fichiers.
- **Reconnaissance intelligente** (En cours) : `Movie (2019).mkv` et `S01E02` deviennent de vrais titres, saisons et épisodes.
- **Affiches et synopsis** (En cours) : Visuels, casting et résumés depuis TMDB et fanart.tv, dans la langue de chaque bibliothèque.
- **Recherche et collections** (Prévu) : Trouve n'importe quoi en une frappe, regroupe tes films comme tu veux.

*Lecture*
- **Direct Play** (Prochainement) : Quand ton appareil sait lire le fichier tel quel, PLAID l'envoie intact. Qualité maximale, zéro CPU dépensé.
- **Remux et transcodage** (Prochainement) : Quand il ne sait pas, PLAID l'adapte à la volée avec FFmpeg.
- **Accélération matérielle** (Prochainement) : VAAPI, Intel Quick Sync, NVENC et VideoToolbox, pour qu'une petite machine puisse quand même transcoder.
- **Sous-titres** (Prévu) : Les sous-titres texte partout, les sous-titres image incrustés quand un appareil ne sait pas les afficher.
- **Reprise partout** (Prévu) : Arrête sur la télé, reprends sur ton téléphone. « Reprendre la lecture » te suit.

*Les gens*
- **Comptes et profils** (Disponible) : Un admin, des comptes locaux, et un profil par personne.
- **Partage de bibliothèques** (Prévu) : Voir le bloc Partage ci-dessous.
- **Plaid Party** (Prévu) : Regarder ensemble, synchronisés. Voir la page Plaid Party.

**Bloc : le partage, sans intermédiaire**
- H2 : Partage une bibliothèque, pas tout ton serveur.
- Body : Envoie à un ami un lien d'invitation à usage unique. Il crée un compte sur ton serveur et ne voit jamais que ce que tu as partagé : une bibliothèque, ou une collection.
- Bullets : Partage par bibliothèque ou par collection · Qualité maximale par ami · Téléchargements autorisés ou non · Nombre de lectures simultanées · Révocation immédiate, lectures en cours comprises

**Bloc : tableau comparatif**
- H2 : PLAID face aux autres.
- Columns : PLAID · Serveurs multimédias hébergés classiques
- Rows : Compte central obligatoire (Non / Souvent oui) · Fonctionne avec une simple URL publique ou un tunnel (Oui / Ça dépend) · Visionnage à plusieurs intégré (Oui, prévu / Module ou rien) · Code source (MIT / Fermé ou partiel) · Télémétrie (Aucune / Ça dépend)
- Footnote : « Prévu » veut dire conçu, pas encore livré. Voir la roadmap.

---

## Page 3 : Plaid Party (`/fr/party`)

**Hero**
- Eyebrow : La fonctionnalité phare · Prévu
- H1 : Même film. Même seconde. Pas le même canapé.
- Lead : Une Plaid Party garde tout le monde sur la même image, où qu'on soit et quel que soit l'écran.
- CTA : Suivre l'avancée sur GitHub

**Comment ça marche**
1. **Lance une party.** Choisis un film, appuie sur « Plaid Party ». Tu obtiens un lien et un code court.
2. **Envoie le lien.** Tes amis rejoignent depuis une télé, un ordi ou un téléphone. Tu vois qui est là et qui charge encore.
3. **Appuie sur play, une fois.** Lecture, pause et avance rapide arrivent chez tout le monde en même temps, à une demi-seconde près.

**Section : chacun son flux**
- H2 : La 4K sur la télé. Le 720p dans le train.
- Body : Tout le monde regarde le même moment, mais chaque appareil reçoit le flux qui lui convient : Direct Play sur la télé du salon, un transcodage plus léger sur un téléphone en 4G. Personne n'attend la connexion la plus lente.

**Section : une synchro qu'on ne remarque pas**
- H2 : Pas de saut, pas de « attends, t'en es où ? »
- Body : Le serveur tient l'horloge. Quand quelqu'un dérive, PLAID ajuste légèrement sa vitesse plutôt que de sauter. Après une mise en mémoire tampon, il se recale tout seul.

**Section : le côté social**
- **Réactions** : Des emoji qui flottent sur le film, pour tout le monde.
- **Chat** : Un panneau latéral qui ne cache jamais l'image.
- **Pause pipi** : N'importe qui peut demander une pause. L'hôte décide, ou tout le monde, à toi de voir.
- **Qui contrôle** : « Hôte seulement » ou « Tout le monde ». Modifiable à tout moment.

**Party room mock strings :** En train de regarder · Chargement… · En pause · « Sam demande une pause » · Accepter · Continuer

**Closing line :** La visio et la voix restent hors de la v1, exprès. Tu as sûrement déjà un appel en cours.

---

## Page 4 : Installer (`/fr/install`)

**Hero**
- H1 : Prêt en cinq minutes.
- Lead : Choisis la méthode qui convient à ta machine. Toutes finissent pareil : ouvre `http://<ton-serveur>:32500`.
- Note : PLAID est en tout début de développement. En attendant la première version publique, le guide d'installation explique comment construire les paquets toi-même.

**Tabs : Ubuntu / Debian (script) · Paquet Debian · Docker**

*Script*
- Text : Installe un service système qui démarre avec la machine. Pas besoin de Docker.

*Paquet Debian*
- Steps : 1. Télécharge `plaid_<version>_amd64.deb` (ou `_arm64.deb`) et `checksums.txt` depuis la page des versions. 2. Vérifie-le (facultatif). 3. Installe-le.
- « Ce que tu obtiens » : `/usr/bin/plaid` le serveur · `/etc/plaid/plaid.env` la configuration, conservée lors des mises à jour · `/var/lib/plaid` tes données, jamais supprimées automatiquement · `plaid.service` activé et démarré

*Docker*
- Text : Monte tes dossiers de médias en lecture seule dans `docker-compose.yml`.

**Code block states :** Copier · Copié

**Section : prérequis**
- Systèmes : Ubuntu 24.04 LTS et 26.04 LTS, Debian 12 et plus récent
- Architectures : amd64 (la plupart des PC), arm64 (Raspberry Pi 4 et 5 en 64 bits, serveurs ARM)
- Installé avec : FFmpeg, depuis ta distribution

**Section : et ensuite**
- **Crée le compte admin** : la première visite te guide.
- **Ajoute tes dossiers** : films et séries, chacun dans sa langue.
- **Lis le guide complet** : configuration, mises à jour, droits sur les dossiers. → docs/install.md sur GitHub

**Empty state :** « Rien à regarder ? Ajoute un dossier et reviens te caler. »

---

## Page 5 : Roadmap (`/fr/roadmap`)

**Hero**
- H1 : Où on en est, honnêtement.
- Lead : PLAID se construit au grand jour, une fonctionnalité à la fois. Chacune commence par une intention écrite, puis une spec, puis un plan, tout est sur GitHub.

**Status labels :** Terminé · En cours · Prochainement · Prévu

**Timeline**
- Phase 0, Fondations : contrat d'API, CI, Docker, paquet Ubuntu/Debian (Terminé) · Design system (En cours)
- Phase 1, Serveur : comptes et authentification (Terminé) · bibliothèques et scan (En cours) · métadonnées et visuels (En cours) · lecture : Direct Play et transcodage (Prochainement)
- Phase 2, App web : accueil et administration (Prévu) · navigation et lecteur (Prévu)
- Phase 3, Ensemble : partage de bibliothèques (Prévu) · Plaid Party (Prévu)
- Phase 4, Apple : iPhone, iPad, Apple TV (Prévu)
- Phase 5, Android : Android et Android TV (Prévu)
- Plus tard : téléchargements hors ligne, Plaid Hub, musique, photos, saut de l'intro et du générique

**Callout :** Envie de voir comment une fonctionnalité est décidée ? Chaque intention, spec et plan vit dans `sdlc/` sur GitHub. → Parcourir les specs

---

## Page 6 : Open source (`/fr/open-source`)

**Hero**
- H1 : Ouvert, lisible, et à toi de le forker.
- Lead : PLAID est sous licence MIT. Tout : le serveur, l'app web, et les apps à venir.

**Section : notre façon de construire**
- **Pas de code sans plan.** Chaque fonctionnalité passe par intention → spec → plan → code, chaque étape relue.
- **Un seul contrat d'API.** L'API est décrite une fois en OpenAPI ; le serveur et chaque client en sont générés.
- **Chaque mot traduisible.** L'anglais d'abord, le français ensuite, et de la place pour ta langue.

**Section : contribuer**
- Body : Un bug, une idée, une langue qu'on ne parle pas ? Ouvre une issue ou une discussion. Les traductions sont une super première contribution.
- Buttons : Ouvrir une issue · Lire le guide de contribution

---

## Page 7 : FAQ (`/fr/faq`)

*Les bases*
- **PLAID est-il gratuit ?** Oui. Il est open source sous licence MIT, sans offre payante.
- **Je peux l'utiliser aujourd'hui ?** Le serveur s'installe et tourne, avec les comptes et le scan des bibliothèques. La lecture et les apps arrivent ensuite. Suis la roadmap pour les dates à mesure qu'elles se précisent.
- **C'est un fork de Plex ou de Jellyfin ?** Non. PLAID est écrit de zéro, avec sa propre API et ses propres apps.

*Tes données*
- **Faut-il un compte en ligne ?** Non. Les comptes vivent sur ton serveur. On ne te demande rien.
- **Est-ce qu'il nous envoie quelque chose ?** Aucune télémétrie. Le serveur ne sort que pour récupérer affiches et synopsis sur TMDB et fanart.tv.
- **Où sont mes fichiers ?** Exactement là où ils étaient. PLAID lit tes dossiers ; il ne déplace ni ne modifie jamais tes médias.

*Matériel*
- **Ça tourne sur un Raspberry Pi ?** Oui, un Pi 4 ou 5 avec un système 64 bits. Pour du transcodage intensif, une machine avec accélération matérielle aide.
- **Faut-il Docker ?** Non. Le paquet Debian installe un service système classique.

*Regarder avec des amis*
- **Comment mes amis accèdent à mon serveur depuis l'extérieur ?** Via une adresse publique, un reverse proxy, ou un tunnel comme Tailscale ou Cloudflare Tunnel. Le guide d'installation explique tout.
- **Mes amis doivent-ils installer quelque chose ?** Un navigateur suffit. Les apps viendront plus tard.

**Bottom card :** Encore une question ? Pose-la sur GitHub Discussions.

---

## Page 8 : 404 (FR)

- H1 : Cette page est partie faire du pop-corn.
- Body : Elle n'est pas là, et elle ne reviendra peut-être pas.
- Button : Retour au canapé

---

## Deliverables, in order
1. Home in dark mode, desktop.
2. The 7 other pages in dark mode, desktop.
3. Home and Plaid Party in light mode.
4. Home, Install and FAQ on phone (375), including the open mobile menu.
5. Home, Install and FAQ in French (desktop and phone), to prove the layouts hold with longer text; the language switch shows FR as active.
6. States sheet: header (scrolled / not), buttons (default, hover, focus, pressed), status labels, code block (default / copied), FAQ accordion (closed / open), install tabs.
