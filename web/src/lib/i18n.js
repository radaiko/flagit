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
