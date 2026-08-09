/**
 * Flagit translations.
 *
 * Deliberately not a library: the overlay ships inside a Go binary and runs in
 * app webviews, so every kilobyte and every runtime dependency is a cost. A
 * flat key/value map and a `t()` lookup covers everything Flagit needs.
 *
 * German is written as German, not as translated English — the register is the
 * same plain, direct voice in both.
 */

export const LANGUAGES = ['en', 'de'];
export const DEFAULT_LANGUAGE = 'en';

const STORAGE_KEY = 'flagit.lang';

export const translations = {
  en: {
    // Chrome
    'app.name': 'Flagit',
    'app.overlayTagline': 'Report a problem',
    'lang.switchTo': 'Auf Deutsch umschalten',
    'lang.label': 'Language',

    // Create ticket
    'create.heading': 'What went wrong?',
    'create.headingFeature': 'What would you like to see?',
    'create.kindLegend': 'Report type',
    'create.bug': 'Bug',
    'create.feature': 'Feature',
    'create.titleLabel': 'Summary',
    'create.titlePlaceholder': 'The app closes when I tap save',
    'create.bodyLabel': 'What happened',
    'create.bodyPlaceholderBug': 'What you did, what you expected, what happened instead.',
    'create.bodyPlaceholderFeature': 'What you want to do, and why the app makes it hard today.',
    'create.diagnosticsLabel': 'Attach diagnostics',
    'create.diagnosticsHint':
      'App version, operating system and the last few minutes of logs. No names, no contact details, no account data.',
    'create.submit': 'Send report',
    'create.submitting': 'Sending…',
    'create.lookupPrompt': 'Already reported something?',
    'create.lookupLink': 'Look up a ticket',

    // Success
    'success.heading': 'Report filed',
    'success.keepId':
      'Keep this ID. It is how you check back — there is no account and no email.',
    'success.copy': 'Copy ID',
    'success.copied': 'Copied',
    'success.check': 'Check status',
    'success.another': 'File another report',

    // Look up / view
    'view.heading': 'Look up a ticket',
    'view.idLabel': 'Ticket ID',
    'view.idPlaceholder': 'FLG-7X3K9Q',
    'view.submit': 'Open ticket',
    'view.loading': 'Opening…',
    'view.back': 'Back',
    'view.reportedOn': 'Filed',
    'view.updatedOn': 'Updated',
    'view.appVersion': 'Version',
    'view.noMessages': 'No replies yet. You will see updates here as work happens.',
    'view.conversation': 'Conversation',
    'view.you': 'You',
    'view.team': 'Flagit',

    // Reply
    'reply.label': 'Add to this report',
    'reply.placeholder': 'Anything else that might help?',
    'reply.submit': 'Send reply',
    'reply.submitting': 'Sending…',
    'reply.sent': 'Reply sent',

    // Status
    'status.open': 'Open',
    'status.in-progress': 'In progress',
    'status.resolved': 'Resolved',
    'status.shipped': 'Shipped',
    'status.closed': 'Closed',
    'status.label': 'Status',

    // Types
    'type.bug': 'Bug',
    'type.feature': 'Feature',

    // Errors
    'error.titleRequired': 'Add a short summary so this is findable.',
    'error.bodyRequired': 'Describe what happened.',
    'error.idRequired': 'Enter a ticket ID.',
    'error.replyRequired': 'Write a reply before sending.',
    'error.notFound': 'No ticket with that ID.',
    'error.forbidden':
      'That ticket belongs to another device. Only the device that filed a ticket can open it.',
    'error.network': 'Could not reach the server. Check your connection and try again.',
    'error.generic': 'Something went wrong. Try again.',

    // Dashboard chrome
    'admin.title': 'Flagit admin',
    'admin.login': 'Sign in',
    'admin.keyLabel': 'Admin key',
    'admin.keyHint': 'Printed once when the server first started.',
    'admin.keyRequired': 'Enter the admin key.',
    'admin.keyInvalid': 'That key was rejected. Check it and try again.',
    'admin.signOut': 'Sign out',
    'admin.navTickets': 'Tickets',
    'admin.navSettings': 'Settings',
    'admin.navHelp': 'Help',
    'admin.commitLabel': 'Commit',
    'admin.commitUnknown': 'unknown',
    'admin.commitCopy': 'Copy commit SHA',
    'admin.commitCopyShort': 'Copy',
    'admin.commitCopied': 'Copied',

    // Ticket list
    'list.heading': 'Tickets',
    'list.empty': 'No tickets yet. They appear here the moment someone flags something.',
    'list.emptyFiltered': 'No tickets match these filters.',
    'list.filterApp': 'App',
    'list.filterStatus': 'Status',
    'list.filterType': 'Type',
    'list.filterAll': 'All',
    'list.colId': 'ID',
    'list.colTitle': 'Title',
    'list.colApp': 'App',
    'list.colStatus': 'Status',
    'list.colCreated': 'Filed',
    'list.select': 'Select ticket',
    'list.selectAll': 'Select all',
    'list.selectedCount': 'selected',
    'list.loading': 'Loading tickets…',
    'list.refresh': 'Refresh',

    // Mass operations
    'mass.heading': 'Update selected',
    'mass.status': 'Set status to',
    'mass.version': 'Shipped in version',
    'mass.versionPlaceholder': '1.5.0',
    'mass.apply': 'Apply',
    'mass.applying': 'Applying…',
    'mass.clear': 'Clear selection',
    'mass.done': 'Updated {n} tickets',
    'mass.partial': 'Updated {n} tickets, {failed} could not be updated',

    // Ticket detail
    'detail.back': 'All tickets',
    'detail.diagnostics': 'Diagnostics',
    'detail.logs': 'Logs',
    'detail.noLogs': 'No logs attached.',
    'detail.commits': 'Commits',
    'detail.noCommits': 'No commits recorded yet.',
    'detail.branch': 'Branch',
    'detail.setStatus': 'Set status',
    'detail.save': 'Save',
    'detail.saving': 'Saving…',
    'detail.saved': 'Ticket updated',
    'detail.replyLabel': 'Reply to reporter',
    'detail.replyPlaceholder': 'This goes straight to the person who filed it.',
    'detail.reply': 'Send reply',
    'detail.device': 'Device',
    'detail.platform': 'Platform',
    'detail.os': 'OS',
    'detail.shippedIn': 'Shipped in',

    // Settings
    'settings.heading': 'Settings',
    'settings.autoHeading': 'Automatic processing',
    'settings.globalAuto': 'Process tickets from new apps automatically',
    'settings.globalAutoHint':
      'Applies the first time Flagit sees an app. After that the per-app switch below decides.',
    'settings.webhookLabel': 'Hermes webhook URL',
    'settings.webhookHint':
      'Where new tickets are sent for automatic processing. Leave empty to handle everything by hand.',
    'settings.webhookInvalid': 'The URL must start with http:// or https://.',
    'settings.save': 'Save settings',
    'settings.saving': 'Saving…',
    'settings.saved': 'Settings saved',
    'settings.appsHeading': 'Apps',
    'settings.appsHint': 'An app appears here after its first ticket.',
    'settings.appsEmpty': 'No apps yet.',
    'settings.appAuto': 'Process automatically',
    'settings.appSince': 'Known since',

    // Help — chrome
    'help.title': 'Help',
    'help.open': 'Help',
    'help.back': 'Back to reporting',
    'help.overlayIntro':
      'How reporting works, and what happens to a report once you have sent it.',
    'help.adminIntro':
      'How Flagit behaves once tickets arrive: the workflow, the switches, and what talks to what.',

    // Help — what each status means. Shared by both help screens, because the
    // reporter and the admin are looking at the same five words.
    'help.status.open': 'Filed, waiting to be picked up.',
    'help.status.in-progress': 'Someone — or an agent — is working on it.',
    'help.status.resolved': 'Fixed in the code, not yet in a release you can install.',
    'help.status.shipped': 'The fix is out. The version it went into is shown on the ticket.',
    'help.status.closed': 'No further work planned. A reply can still restart the conversation.',

    // Help — overlay
    'help.overlay.report.title': 'Filing a report',
    'help.overlay.report.body1':
      'Pick Bug if something is broken, or Feature if something is missing. Add a one-line summary, then describe what you did, what you expected, and what happened instead.',
    'help.overlay.report.body2':
      'Diagnostics — app version, operating system and the last few minutes of logs — are attached by default. Clear the checkbox and nothing but your own text is sent.',

    'help.overlay.id.title': 'Your ticket ID',
    'help.overlay.id.body1':
      'Every report gets an ID like FLG-7X3K9Q. It appears the moment the report is filed, and it is the only way back to it: no account, no email address, no password.',
    'help.overlay.id.body2':
      'Keep it somewhere safe. To check on a report later, open this panel, choose “Look up a ticket” and enter the ID.',

    'help.overlay.replies.title': 'Replies',
    'help.overlay.replies.body1':
      'Anything the team or the agent writes lands in the ticket’s conversation. Open the ticket with its ID to read it — nothing is pushed to you, because Flagit has no way to reach you.',
    'help.overlay.replies.body2':
      'You can write back from the same screen. Extra detail, exact steps, or a note that the problem has stopped happening all help.',

    'help.overlay.device.title': 'The device token',
    'help.overlay.device.body1':
      'The first time you file something, Flagit stores a random token in this browser. That token is what proves a ticket is yours, and it stands in for a login.',
    'help.overlay.device.body2':
      'Only a one-way hash of it ever reaches the server. Clearing site data, switching browsers or moving to another device means older tickets can no longer be opened — so keep the IDs for the reports you care about.',

    'help.overlay.status.title': 'What the statuses mean',
    'help.overlay.status.body1':
      'A ticket carries one of five statuses. It is shown next to the ID every time you open it.',

    // Help — dashboard
    'help.admin.workflow.title': 'Status workflow',
    'help.admin.workflow.body1':
      'The documented path is open → in progress → resolved → shipped → closed. A ticket can also jump straight to closed from any stage, or step back one stage when work reopens or a release is rolled back.',
    'help.admin.workflow.body2':
      'Anything else is rejected as a slip. When you genuinely mean it, the mass update forces the change through; over the API the same call takes "force": true.',

    'help.admin.apps.title': 'Per-app settings',
    'help.admin.apps.body1':
      'An app appears under Settings after its first ticket. Its own switch decides whether new tickets from that app are handed to Hermes automatically.',
    'help.admin.apps.body2':
      'The global switch above it only applies the first time Flagit sees an app, and sets the starting value of that app’s switch. From then on the app decides.',

    'help.admin.mass.title': 'Mass operations',
    'help.admin.mass.body1':
      'Select tickets in the list and use “Update selected” to move them in one go. This is the release sweep: tick everything that went out, set the status to Shipped, and type the version.',
    'help.admin.mass.body2':
      'The version is stored on each ticket and shown to the reporter, so “fixed in 1.5.0” needs no further message. The result says how many tickets moved and names any that did not.',

    'help.admin.hermes.title': 'Hermes integration',
    'help.admin.hermes.body1':
      'When a ticket is created and automatic processing is on, Flagit POSTs it to the Hermes webhook URL from Settings. Delivery is retried up to three times with exponential backoff; a 4xx counts as final.',
    'help.admin.hermes.body2':
      'Hermes can also pull. GET /internal/poll?since=<timestamp> returns everything created or updated since that moment, so a missed webhook is never a lost ticket.',

    'help.admin.commits.title': 'Commit tracking',
    'help.admin.commits.body1':
      'Agents record the commits they produce against the ticket. Hash, branch and message appear under Commits on the ticket detail.',
    'help.admin.commits.body2':
      'This is developer-facing only: the public API never returns commits, and the reporter never sees them.',

    'help.admin.access.title': 'The admin key',
    'help.admin.access.body1':
      'This dashboard and every /internal endpoint are gated by a single key, sent as the X-Admin-Key header. It is generated and printed once on first start; only its hash is stored.',
    'help.admin.access.body2':
      'The key lives in this tab’s session storage, so closing the tab signs you out. Keep the internal port on a private network — a tailnet, or loopback plus an SSH tunnel — rather than the open internet.',
  },

  de: {
    // Chrome
    'app.name': 'Flagit',
    'app.overlayTagline': 'Problem melden',
    'lang.switchTo': 'Switch to English',
    'lang.label': 'Sprache',

    // Create ticket
    'create.heading': 'Was ist schiefgelaufen?',
    'create.headingFeature': 'Was wünschst du dir?',
    'create.kindLegend': 'Art der Meldung',
    'create.bug': 'Fehler',
    'create.feature': 'Wunsch',
    'create.titleLabel': 'Kurzfassung',
    'create.titlePlaceholder': 'Die App schließt sich beim Speichern',
    'create.bodyLabel': 'Was passiert ist',
    'create.bodyPlaceholderBug':
      'Was du gemacht hast, was du erwartet hast, was stattdessen passiert ist.',
    'create.bodyPlaceholderFeature':
      'Was du machen möchtest und warum die App es dir heute schwer macht.',
    'create.diagnosticsLabel': 'Diagnosedaten mitschicken',
    'create.diagnosticsHint':
      'App-Version, Betriebssystem und die letzten Minuten aus dem Protokoll. Keine Namen, keine Kontaktdaten, keine Kontodaten.',
    'create.submit': 'Meldung senden',
    'create.submitting': 'Wird gesendet…',
    'create.lookupPrompt': 'Schon einmal etwas gemeldet?',
    'create.lookupLink': 'Ticket nachschlagen',

    // Success
    'success.heading': 'Meldung ist eingegangen',
    'success.keepId':
      'Bewahre diese ID auf. Damit schaust du später nach — ohne Konto und ohne E-Mail.',
    'success.copy': 'ID kopieren',
    'success.copied': 'Kopiert',
    'success.check': 'Status ansehen',
    'success.another': 'Weitere Meldung senden',

    // Look up / view
    'view.heading': 'Ticket nachschlagen',
    'view.idLabel': 'Ticket-ID',
    'view.idPlaceholder': 'FLG-7X3K9Q',
    'view.submit': 'Ticket öffnen',
    'view.loading': 'Wird geöffnet…',
    'view.back': 'Zurück',
    'view.reportedOn': 'Gemeldet',
    'view.updatedOn': 'Aktualisiert',
    'view.appVersion': 'Version',
    'view.noMessages':
      'Noch keine Antworten. Sobald sich etwas tut, siehst du es hier.',
    'view.conversation': 'Verlauf',
    'view.you': 'Du',
    'view.team': 'Flagit',

    // Reply
    'reply.label': 'Etwas ergänzen',
    'reply.placeholder': 'Gibt es noch etwas, das weiterhilft?',
    'reply.submit': 'Antwort senden',
    'reply.submitting': 'Wird gesendet…',
    'reply.sent': 'Antwort gesendet',

    // Status
    'status.open': 'Offen',
    'status.in-progress': 'In Arbeit',
    'status.resolved': 'Behoben',
    'status.shipped': 'Ausgeliefert',
    'status.closed': 'Geschlossen',
    'status.label': 'Status',

    // Types
    'type.bug': 'Fehler',
    'type.feature': 'Wunsch',

    // Errors
    'error.titleRequired': 'Schreib eine kurze Zusammenfassung, damit die Meldung auffindbar ist.',
    'error.bodyRequired': 'Beschreib, was passiert ist.',
    'error.idRequired': 'Gib eine Ticket-ID ein.',
    'error.replyRequired': 'Schreib eine Antwort, bevor du sendest.',
    'error.notFound': 'Zu dieser ID gibt es kein Ticket.',
    'error.forbidden':
      'Dieses Ticket gehört zu einem anderen Gerät. Nur das Gerät, das es gemeldet hat, kann es öffnen.',
    'error.network': 'Der Server ist nicht erreichbar. Prüf deine Verbindung und versuch es erneut.',
    'error.generic': 'Da ist etwas schiefgelaufen. Versuch es erneut.',

    // Dashboard chrome
    'admin.title': 'Flagit-Verwaltung',
    'admin.login': 'Anmelden',
    'admin.keyLabel': 'Admin-Schlüssel',
    'admin.keyHint': 'Wurde beim ersten Start des Servers einmalig ausgegeben.',
    'admin.keyRequired': 'Gib den Admin-Schlüssel ein.',
    'admin.keyInvalid': 'Der Schlüssel wurde abgelehnt. Prüf ihn und versuch es erneut.',
    'admin.signOut': 'Abmelden',
    'admin.navTickets': 'Tickets',
    'admin.navSettings': 'Einstellungen',
    'admin.navHelp': 'Hilfe',
    'admin.commitLabel': 'Commit',
    'admin.commitUnknown': 'unbekannt',
    'admin.commitCopy': 'Commit-SHA kopieren',
    'admin.commitCopyShort': 'Kopieren',
    'admin.commitCopied': 'Kopiert',

    // Ticket list
    'list.heading': 'Tickets',
    'list.empty': 'Noch keine Tickets. Sie erscheinen hier, sobald jemand etwas meldet.',
    'list.emptyFiltered': 'Keine Tickets passen zu diesen Filtern.',
    'list.filterApp': 'App',
    'list.filterStatus': 'Status',
    'list.filterType': 'Art',
    'list.filterAll': 'Alle',
    'list.colId': 'ID',
    'list.colTitle': 'Titel',
    'list.colApp': 'App',
    'list.colStatus': 'Status',
    'list.colCreated': 'Gemeldet',
    'list.select': 'Ticket auswählen',
    'list.selectAll': 'Alle auswählen',
    'list.selectedCount': 'ausgewählt',
    'list.loading': 'Tickets werden geladen…',
    'list.refresh': 'Aktualisieren',

    // Mass operations
    'mass.heading': 'Auswahl bearbeiten',
    'mass.status': 'Status setzen auf',
    'mass.version': 'Ausgeliefert in Version',
    'mass.versionPlaceholder': '1.5.0',
    'mass.apply': 'Übernehmen',
    'mass.applying': 'Wird übernommen…',
    'mass.clear': 'Auswahl aufheben',
    'mass.done': '{n} Tickets aktualisiert',
    'mass.partial': '{n} Tickets aktualisiert, {failed} konnten nicht aktualisiert werden',

    // Ticket detail
    'detail.back': 'Alle Tickets',
    'detail.diagnostics': 'Diagnosedaten',
    'detail.logs': 'Protokoll',
    'detail.noLogs': 'Kein Protokoll mitgeschickt.',
    'detail.commits': 'Commits',
    'detail.noCommits': 'Noch keine Commits erfasst.',
    'detail.branch': 'Branch',
    'detail.setStatus': 'Status setzen',
    'detail.save': 'Speichern',
    'detail.saving': 'Wird gespeichert…',
    'detail.saved': 'Ticket aktualisiert',
    'detail.replyLabel': 'Der meldenden Person antworten',
    'detail.replyPlaceholder': 'Das geht direkt an die Person, die gemeldet hat.',
    'detail.reply': 'Antwort senden',
    'detail.device': 'Gerät',
    'detail.platform': 'Plattform',
    'detail.os': 'Betriebssystem',
    'detail.shippedIn': 'Ausgeliefert in',

    // Settings
    'settings.heading': 'Einstellungen',
    'settings.autoHeading': 'Automatische Bearbeitung',
    'settings.globalAuto': 'Tickets neuer Apps automatisch bearbeiten',
    'settings.globalAutoHint':
      'Gilt beim ersten Mal, wenn Flagit eine App sieht. Danach entscheidet der Schalter bei der App selbst.',
    'settings.webhookLabel': 'Hermes-Webhook-URL',
    'settings.webhookHint':
      'Wohin neue Tickets zur automatischen Bearbeitung geschickt werden. Leer lassen, um alles von Hand zu erledigen.',
    'settings.webhookInvalid': 'Die URL muss mit http:// oder https:// beginnen.',
    'settings.save': 'Einstellungen speichern',
    'settings.saving': 'Wird gespeichert…',
    'settings.saved': 'Einstellungen gespeichert',
    'settings.appsHeading': 'Apps',
    'settings.appsHint': 'Eine App erscheint hier nach ihrem ersten Ticket.',
    'settings.appsEmpty': 'Noch keine Apps.',
    'settings.appAuto': 'Automatisch bearbeiten',
    'settings.appSince': 'Bekannt seit',

    // Hilfe — Rahmen
    'help.title': 'Hilfe',
    'help.open': 'Hilfe',
    'help.back': 'Zurück zum Melden',
    'help.overlayIntro':
      'Wie das Melden funktioniert und was mit einer Meldung passiert, nachdem du sie abgeschickt hast.',
    'help.adminIntro':
      'Wie sich Flagit verhält, sobald Tickets ankommen: der Ablauf, die Schalter und was womit spricht.',

    // Hilfe — was die einzelnen Status bedeuten
    'help.status.open': 'Gemeldet und wartet darauf, dass sich jemand darum kümmert.',
    'help.status.in-progress': 'Jemand — oder ein Agent — arbeitet daran.',
    'help.status.resolved': 'Im Code behoben, aber noch in keiner Version, die du installieren kannst.',
    'help.status.shipped': 'Die Behebung ist draußen. Am Ticket steht, in welcher Version.',
    'help.status.closed':
      'Es ist keine weitere Arbeit geplant. Eine Antwort kann das Gespräch wieder in Gang bringen.',

    // Hilfe — Overlay
    'help.overlay.report.title': 'Eine Meldung schreiben',
    'help.overlay.report.body1':
      'Wähl „Fehler“, wenn etwas kaputt ist, oder „Wunsch“, wenn etwas fehlt. Schreib eine Zeile als Zusammenfassung und danach, was du gemacht hast, was du erwartet hast und was stattdessen passiert ist.',
    'help.overlay.report.body2':
      'Diagnosedaten — App-Version, Betriebssystem und die letzten Minuten aus dem Protokoll — gehen standardmäßig mit. Nimm den Haken weg, dann wird nur dein eigener Text gesendet.',

    'help.overlay.id.title': 'Deine Ticket-ID',
    'help.overlay.id.body1':
      'Jede Meldung bekommt eine ID wie FLG-7X3K9Q. Sie erscheint, sobald die Meldung eingegangen ist, und sie ist der einzige Weg zurück: kein Konto, keine E-Mail-Adresse, kein Passwort.',
    'help.overlay.id.body2':
      'Heb sie gut auf. Zum Nachschauen öffnest du später dieses Fenster, wählst „Ticket nachschlagen“ und gibst die ID ein.',

    'help.overlay.replies.title': 'Antworten',
    'help.overlay.replies.body1':
      'Alles, was das Team oder der Agent schreibt, landet im Verlauf des Tickets. Öffne das Ticket mit seiner ID, um es zu lesen — von selbst kommt nichts bei dir an, denn Flagit hat keine Möglichkeit, dich zu erreichen.',
    'help.overlay.replies.body2':
      'Auf demselben Bildschirm kannst du zurückschreiben. Zusätzliche Details, die genauen Schritte oder der Hinweis, dass das Problem nicht mehr auftritt, helfen alle weiter.',

    'help.overlay.device.title': 'Der Geräte-Token',
    'help.overlay.device.body1':
      'Beim ersten Melden legt Flagit einen zufälligen Token in diesem Browser ab. Dieser Token belegt, dass ein Ticket dir gehört, und ersetzt die Anmeldung.',
    'help.overlay.device.body2':
      'Auf dem Server landet nur ein Hash davon. Wenn du die Websitedaten löschst, den Browser wechselst oder auf ein anderes Gerät gehst, lassen sich ältere Tickets nicht mehr öffnen — heb die IDs also auf, wenn dir eine Meldung wichtig ist.',

    'help.overlay.status.title': 'Was die Status bedeuten',
    'help.overlay.status.body1':
      'Ein Ticket hat immer einen von fünf Status. Er steht bei jedem Öffnen direkt neben der ID.',

    // Hilfe — Verwaltung
    'help.admin.workflow.title': 'Der Statusablauf',
    'help.admin.workflow.body1':
      'Der dokumentierte Weg ist offen → in Arbeit → behoben → ausgeliefert → geschlossen. Ein Ticket darf aus jeder Stufe direkt auf „geschlossen“ springen oder eine Stufe zurückgehen, wenn die Arbeit wieder aufgenommen oder ein Release zurückgerollt wird.',
    'help.admin.workflow.body2':
      'Alles andere wird als Versehen abgelehnt. Wenn du es wirklich so meinst, erzwingt die Sammelbearbeitung die Änderung; über die API macht das "force": true im selben Aufruf.',

    'help.admin.apps.title': 'Einstellungen je App',
    'help.admin.apps.body1':
      'Eine App taucht unter „Einstellungen“ auf, sobald ihr erstes Ticket da ist. Ihr eigener Schalter entscheidet, ob neue Tickets dieser App automatisch an Hermes gehen.',
    'help.admin.apps.body2':
      'Der globale Schalter darüber greift nur beim ersten Kontakt mit einer App und legt den Startwert für deren Schalter fest. Danach entscheidet die App.',

    'help.admin.mass.title': 'Sammelbearbeitung',
    'help.admin.mass.body1':
      'Wähl Tickets in der Liste aus und bearbeite sie mit „Auswahl bearbeiten“ in einem Zug. Das ist der Release-Durchgang: alles anhaken, was rausgegangen ist, den Status auf „Ausgeliefert“ setzen und die Version eintragen.',
    'help.admin.mass.body2':
      'Die Version steht danach am Ticket und ist für die meldende Person sichtbar — „behoben in 1.5.0“ braucht also keine weitere Nachricht. Das Ergebnis nennt die Zahl der geänderten Tickets und die, bei denen es nicht geklappt hat.',

    'help.admin.hermes.title': 'Hermes-Anbindung',
    'help.admin.hermes.body1':
      'Wenn ein Ticket entsteht und die automatische Bearbeitung aktiv ist, schickt Flagit es per POST an die Hermes-Webhook-URL aus den Einstellungen. Es wird bis zu dreimal mit wachsendem Abstand erneut versucht; ein 4xx gilt als endgültig.',
    'help.admin.hermes.body2':
      'Hermes kann auch selbst nachfragen: GET /internal/poll?since=<Zeitstempel> liefert alles, was seitdem entstanden oder geändert wurde. Ein verpasster Webhook ist damit nie ein verlorenes Ticket.',

    'help.admin.commits.title': 'Commits verfolgen',
    'help.admin.commits.body1':
      'Agenten tragen die Commits, die sie erzeugen, am Ticket ein. Hash, Branch und Nachricht stehen dann in der Ticketansicht unter „Commits“.',
    'help.admin.commits.body2':
      'Das ist nur für die Entwicklung gedacht: Die öffentliche API gibt Commits nie aus, und die meldende Person sieht sie nicht.',

    'help.admin.access.title': 'Der Admin-Schlüssel',
    'help.admin.access.body1':
      'Diese Verwaltung und jeder /internal-Endpunkt hängen an einem einzigen Schlüssel, der als Header X-Admin-Key mitgeht. Er wird beim ersten Start einmalig erzeugt und ausgegeben; gespeichert wird nur sein Hash.',
    'help.admin.access.body2':
      'Der Schlüssel liegt im Session-Speicher dieses Tabs — Tab zu heißt abgemeldet. Betreib den internen Port in einem privaten Netz (Tailnet oder localhost plus SSH-Tunnel) statt im offenen Internet.',
  },
};

/**
 * Look up a translation.
 *
 * Falls back to English, then to the key itself, so a missing string shows up
 * as an obvious `create.heading` in the UI rather than as blank space.
 *
 * @param {string} key
 * @param {string} [lang]
 * @param {Record<string, string|number>} [vars] `{n}`-style placeholders
 * @returns {string}
 */
export function t(key, lang = DEFAULT_LANGUAGE, vars = null) {
  const table = translations[lang] ?? translations[DEFAULT_LANGUAGE];
  let value = table[key] ?? translations[DEFAULT_LANGUAGE][key] ?? key;

  if (vars) {
    for (const [name, replacement] of Object.entries(vars)) {
      value = value.replaceAll(`{${name}}`, String(replacement));
    }
  }
  return value;
}

/**
 * Normalise anything language-shaped into a supported language code.
 * `de-AT`, `DE` and `de` all mean German.
 *
 * @param {string|null|undefined} tag
 * @returns {string|null}
 */
export function normaliseLanguage(tag) {
  if (typeof tag !== 'string') return null;
  const base = tag.trim().toLowerCase().split('-')[0];
  return LANGUAGES.includes(base) ? base : null;
}

/**
 * Decide which language to open in, in order of intent: an explicit choice in
 * the URL, a choice made earlier in this session, then the browser's own
 * preference. English is the fallback.
 *
 * @param {{search?: string, storage?: Storage, navigatorLanguages?: readonly string[]}} [options]
 * @returns {string}
 */
export function detectLanguage({ search, storage, navigatorLanguages } = {}) {
  const params = new URLSearchParams(search ?? globalThis.location?.search ?? '');
  const fromUrl = normaliseLanguage(params.get('lang'));
  if (fromUrl) return fromUrl;

  const store = storage ?? safeSessionStorage();
  const fromStorage = normaliseLanguage(store?.getItem(STORAGE_KEY));
  if (fromStorage) return fromStorage;

  const preferences = navigatorLanguages ?? globalThis.navigator?.languages ?? [];
  for (const tag of preferences) {
    const match = normaliseLanguage(tag);
    if (match) return match;
  }
  return normaliseLanguage(globalThis.navigator?.language) ?? DEFAULT_LANGUAGE;
}

/**
 * Remember a language choice for the rest of the session.
 *
 * @param {string} lang
 * @param {Storage} [storage]
 */
export function rememberLanguage(lang, storage) {
  const store = storage ?? safeSessionStorage();
  try {
    store?.setItem(STORAGE_KEY, lang);
  } catch {
    // Private browsing modes can refuse to write. The choice still applies for
    // this page view, which is the part the person actually asked for.
  }
}

/**
 * Reflect the active language on <html lang>, so screen readers switch
 * pronunciation and the browser offers the right translation prompt. Svelte
 * only owns the mount point, so this attribute has to be set by hand.
 *
 * @param {string} lang
 */
export function applyDocumentLanguage(lang) {
  const element = globalThis.document?.documentElement;
  if (element) element.lang = lang;
}

/** The other language — the toggle is binary, so "other" is unambiguous. */
export function otherLanguage(lang) {
  return lang === 'de' ? 'en' : 'de';
}

function safeSessionStorage() {
  try {
    return globalThis.sessionStorage ?? null;
  } catch {
    return null;
  }
}
