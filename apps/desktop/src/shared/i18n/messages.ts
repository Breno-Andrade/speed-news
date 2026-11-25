import type { SessionKind } from "../api/types";

export type Messages = {
  appTitle: string;
  connecting: string;
  connectionFailed: string;
  sessionLabels: Record<SessionKind, string>;
  unreadFilter: string;
  allFilter: string;
  markAllRead: string;
  refresh: string;
  settings: string;
  back: string;
  readOnSource: string;
  noUnreadArticles: string;
  noArticles: string;
  loadingArticles: string;
  loadFailed: string;
  raceWeekendNow: string;
  nextRace: string;
  round: (roundNumber: number) => string;
  liveNow: string;
  startsIn: (countdownText: string) => string;
  timeToBeConfirmed: string;
  timesShownIn: (timeZoneDescription: string) => string;
  change: string;
  noUpcomingRace: string;
  timeZoneTitle: string;
  timeZoneAutomatic: (detectedTimeZone: string) => string;
  timeZoneManual: string;
  timeZoneSearchPlaceholder: string;
  saved: string;
  attribution: string;
  interfaceLanguageTitle: string;
  interfaceLanguageAutomatic: (detectedLanguage: string) => string;
  newsLanguagesTitle: string;
  newsLanguagesHint: string;
  trayOpenMenuLabel: string;
  trayQuitMenuLabel: string;
  trayTooltip: (unreadCount: number) => string;
  scheduleTab: string;
  lastRaceTab: string;
  circuitTab: string;
  weatherTab: string;
  loadingPanel: string;
  panelUnavailable: string;
  polePosition: string;
  fastestLap: string;
  onLap: (lapNumber: number) => string;
  startedFrom: (gridPosition: number) => string;
  lapsBehind: (lapCount: number) => string;
  trackLength: string;
  raceLaps: string;
  raceDistance: string;
  firstGrandPrix: string;
  altitude: string;
  previousEditionTitle: (season: number) => string;
  recentWinners: string;
  forecastNotYet: string;
  rainChance: string;
  wind: string;
  weatherLabels: Record<WeatherCondition, string>;
  windowBehaviorTitle: string;
  closeOnFocusLoss: string;
  closeOnFocusLossHint: string;
  readFullArticle: string;
  summaryAttribution: (sourceName: string) => string;
  publishedAtLabel: (dateText: string) => string;
  copyLink: string;
  linkCopied: string;
  markAsUnread: string;
  markAsRead: string;
  previousArticle: string;
  nextArticle: string;
  settingsSaveFailed: string;
  updatedTo: (appVersion: string) => string;
  updateReady: (appVersion: string) => string;
  installUpdate: string;
  installUpdatePasswordHint: string;
  dismiss: string;
};

export type WeatherCondition = "clear" | "mainlyClear" | "partlyCloudy" | "cloudy" | "fog" | "drizzle" | "rain" | "snow" | "showers" | "thunderstorm";

const portugueseMessages: Messages = {
  appTitle: "Speed News",
  connecting: "Conectando ao serviço de notícias…",
  connectionFailed: "Não foi possível iniciar o serviço de notícias.",
  sessionLabels: {
    practice_1: "Treino Livre 1",
    practice_2: "Treino Livre 2",
    practice_3: "Treino Livre 3",
    sprint_qualifying: "Classificação sprint",
    sprint: "Corrida sprint",
    qualifying: "Classificação",
    race: "Corrida",
  },
  unreadFilter: "Não lidas",
  allFilter: "Todas",
  markAllRead: "Marcar todas como lidas",
  refresh: "Atualizar",
  settings: "Configurações",
  back: "Voltar",
  readOnSource: "Ler na fonte",
  noUnreadArticles: "Tudo em dia! Nenhuma notícia nova.",
  noArticles: "Nenhuma notícia ainda.",
  loadingArticles: "Carregando notícias…",
  loadFailed: "Falha ao carregar. Tente atualizar.",
  raceWeekendNow: "Semana de corrida",
  nextRace: "Próxima corrida",
  round: (roundNumber) => `Etapa ${roundNumber}`,
  liveNow: "agora",
  startsIn: (countdownText) => `em ${countdownText}`,
  timeToBeConfirmed: "a confirmar",
  timesShownIn: (timeZoneDescription) => `Fuso: ${timeZoneDescription}`,
  change: "alterar",
  noUpcomingRace: "Nenhuma corrida no calendário.",
  timeZoneTitle: "Fuso horário",
  timeZoneAutomatic: (detectedTimeZone) => `Automático (do computador: ${detectedTimeZone})`,
  timeZoneManual: "Escolher manualmente",
  timeZoneSearchPlaceholder: "Buscar cidade ou região…",
  saved: "Salvo",
  attribution: "Fontes",
  interfaceLanguageTitle: "Idioma do app",
  interfaceLanguageAutomatic: (detectedLanguage) => `Automático (do computador: ${detectedLanguage})`,
  newsLanguagesTitle: "Notícias em",
  newsLanguagesHint: "O contador do ícone considera só esses idiomas.",
  trayOpenMenuLabel: "Abrir notícias",
  trayQuitMenuLabel: "Sair",
  trayTooltip: (unreadCount) => (unreadCount === 0 ? "Speed News" : `Speed News — ${unreadCount} não lida${unreadCount === 1 ? "" : "s"}`),
  scheduleTab: "Programação",
  lastRaceTab: "Último GP",
  circuitTab: "Pista",
  weatherTab: "Clima",
  loadingPanel: "Carregando…",
  panelUnavailable: "Não foi possível carregar agora. Tente de novo em instantes.",
  polePosition: "Pole",
  fastestLap: "Volta mais rápida",
  onLap: (lapNumber) => `volta ${lapNumber}`,
  startedFrom: (gridPosition) => `largou em ${gridPosition}º`,
  lapsBehind: (lapCount) => `+${lapCount} volta${lapCount === 1 ? "" : "s"}`,
  trackLength: "Extensão",
  raceLaps: "Voltas",
  raceDistance: "Distância",
  firstGrandPrix: "1º GP",
  altitude: "Altitude",
  previousEditionTitle: (season) => `Em ${season}`,
  recentWinners: "Vencedores recentes",
  forecastNotYet: "A previsão do tempo aparece a partir de 7 dias antes da corrida.",
  rainChance: "chuva",
  wind: "vento",
  weatherLabels: {
    clear: "Céu limpo",
    mainlyClear: "Poucas nuvens",
    partlyCloudy: "Parcialmente nublado",
    cloudy: "Nublado",
    fog: "Neblina",
    drizzle: "Garoa",
    rain: "Chuva",
    snow: "Neve",
    showers: "Pancadas de chuva",
    thunderstorm: "Tempestade",
  },
  windowBehaviorTitle: "Janela",
  closeOnFocusLoss: "Fechar ao clicar fora",
  closeOnFocusLossHint: "Desligado, a janela fecha só pelo ícone na barra ou pela tecla Esc.",
  readFullArticle: "Ler matéria completa",
  summaryAttribution: (sourceName) => `Resumo publicado por ${sourceName}`,
  publishedAtLabel: (dateText) => `Publicado em ${dateText}`,
  copyLink: "Copiar link",
  linkCopied: "Link copiado",
  markAsUnread: "Marcar como não lida",
  markAsRead: "Marcar como lida",
  previousArticle: "Anterior",
  nextArticle: "Próxima",
  settingsSaveFailed: "Não foi possível salvar",
  updatedTo: (appVersion) => `Atualizado para a versão ${appVersion}`,
  updateReady: (appVersion) => `Versão ${appVersion} pronta`,
  installUpdate: "Atualizar agora",
  installUpdatePasswordHint: "O sistema vai pedir sua senha de administrador.",
  dismiss: "Fechar aviso",
};

const englishMessages: Messages = {
  appTitle: "Speed News",
  connecting: "Connecting to the news service…",
  connectionFailed: "Could not start the news service.",
  sessionLabels: {
    practice_1: "Practice 1",
    practice_2: "Practice 2",
    practice_3: "Practice 3",
    sprint_qualifying: "Sprint Qualifying",
    sprint: "Sprint",
    qualifying: "Qualifying",
    race: "Race",
  },
  unreadFilter: "Unread",
  allFilter: "All",
  markAllRead: "Mark all as read",
  refresh: "Refresh",
  settings: "Settings",
  back: "Back",
  readOnSource: "Read on source",
  noUnreadArticles: "All caught up! No new articles.",
  noArticles: "No articles yet.",
  loadingArticles: "Loading articles…",
  loadFailed: "Failed to load. Try refreshing.",
  raceWeekendNow: "Race weekend",
  nextRace: "Next race",
  round: (roundNumber) => `Round ${roundNumber}`,
  liveNow: "now",
  startsIn: (countdownText) => `in ${countdownText}`,
  timeToBeConfirmed: "TBC",
  timesShownIn: (timeZoneDescription) => `Time zone: ${timeZoneDescription}`,
  change: "change",
  noUpcomingRace: "No race on the calendar.",
  timeZoneTitle: "Time zone",
  timeZoneAutomatic: (detectedTimeZone) => `Automatic (computer: ${detectedTimeZone})`,
  timeZoneManual: "Choose manually",
  timeZoneSearchPlaceholder: "Search city or region…",
  saved: "Saved",
  attribution: "Sources",
  interfaceLanguageTitle: "App language",
  interfaceLanguageAutomatic: (detectedLanguage) => `Automatic (computer: ${detectedLanguage})`,
  newsLanguagesTitle: "News in",
  newsLanguagesHint: "The tray counter only counts these languages.",
  trayOpenMenuLabel: "Open news",
  trayQuitMenuLabel: "Quit",
  trayTooltip: (unreadCount) => (unreadCount === 0 ? "Speed News" : `Speed News — ${unreadCount} unread`),
  scheduleTab: "Schedule",
  lastRaceTab: "Last GP",
  circuitTab: "Track",
  weatherTab: "Weather",
  loadingPanel: "Loading…",
  panelUnavailable: "Couldn't load right now. Try again in a moment.",
  polePosition: "Pole",
  fastestLap: "Fastest lap",
  onLap: (lapNumber) => `lap ${lapNumber}`,
  startedFrom: (gridPosition) => `started P${gridPosition}`,
  lapsBehind: (lapCount) => `+${lapCount} lap${lapCount === 1 ? "" : "s"}`,
  trackLength: "Length",
  raceLaps: "Laps",
  raceDistance: "Distance",
  firstGrandPrix: "First GP",
  altitude: "Altitude",
  previousEditionTitle: (season) => `In ${season}`,
  recentWinners: "Recent winners",
  forecastNotYet: "The weather forecast shows up 7 days before the race.",
  rainChance: "rain",
  wind: "wind",
  weatherLabels: {
    clear: "Clear sky",
    mainlyClear: "Mainly clear",
    partlyCloudy: "Partly cloudy",
    cloudy: "Overcast",
    fog: "Fog",
    drizzle: "Drizzle",
    rain: "Rain",
    snow: "Snow",
    showers: "Rain showers",
    thunderstorm: "Thunderstorm",
  },
  windowBehaviorTitle: "Window",
  closeOnFocusLoss: "Close when clicking outside",
  closeOnFocusLossHint: "When off, the window only closes from the panel icon or the Esc key.",
  readFullArticle: "Read full article",
  summaryAttribution: (sourceName) => `Summary published by ${sourceName}`,
  publishedAtLabel: (dateText) => `Published on ${dateText}`,
  copyLink: "Copy link",
  linkCopied: "Link copied",
  markAsUnread: "Mark as unread",
  markAsRead: "Mark as read",
  previousArticle: "Previous",
  nextArticle: "Next",
  settingsSaveFailed: "Couldn't save",
  updatedTo: (appVersion) => `Updated to version ${appVersion}`,
  updateReady: (appVersion) => `Version ${appVersion} is ready`,
  installUpdate: "Update now",
  installUpdatePasswordHint: "Your system will ask for the administrator password.",
  dismiss: "Dismiss",
};

// Jolpica returns English names; Portuguese names follow how Brazilian media calls each Grand Prix.
const portugueseRaceNames: Record<string, string> = {
  "Australian Grand Prix": "GP da Austrália",
  "Chinese Grand Prix": "GP da China",
  "Japanese Grand Prix": "GP do Japão",
  "Miami Grand Prix": "GP de Miami",
  "Canadian Grand Prix": "GP do Canadá",
  "Monaco Grand Prix": "GP de Mônaco",
  "Barcelona Grand Prix": "GP de Barcelona",
  "Austrian Grand Prix": "GP da Áustria",
  "British Grand Prix": "GP da Inglaterra",
  "Belgian Grand Prix": "GP da Bélgica",
  "Hungarian Grand Prix": "GP da Hungria",
  "Dutch Grand Prix": "GP da Holanda",
  "Italian Grand Prix": "GP da Itália",
  "Spanish Grand Prix": "GP da Espanha",
  "Azerbaijan Grand Prix": "GP do Azerbaijão",
  "Bahrain Grand Prix": "GP do Bahrein",
  "Bahrain Grand Prix in Malaysia": "GP do Bahrein (na Malásia)",
  "Saudi Arabian Grand Prix": "GP da Arábia Saudita",
  "Emilia Romagna Grand Prix": "GP da Emília-Romanha",
  "Singapore Grand Prix": "GP de Singapura",
  "United States Grand Prix": "GP dos Estados Unidos",
  "Mexico City Grand Prix": "GP da Cidade do México",
  "Brazilian Grand Prix": "GP de São Paulo",
  "São Paulo Grand Prix": "GP de São Paulo",
  "Las Vegas Grand Prix": "GP de Las Vegas",
  "Qatar Grand Prix": "GP do Catar",
  "Abu Dhabi Grand Prix": "GP de Abu Dhabi",
};

export type LocalizedContent = {
  locale: string;
  messages: Messages;
  translateRaceName: (officialRaceName: string) => string;
};

/** Turns the saved preference into a locale: "system" follows the operating system. */
export function resolvePreferredLocale(interfaceLanguage: "system" | "pt-BR" | "en", systemLocale: string): string {
  return interfaceLanguage === "system" ? systemLocale : interfaceLanguage;
}

/** "pt" → "Português" in the given locale. */
export function describeLanguage(languageCode: string, displayLocale: string): string {
  const languageName = new Intl.DisplayNames([displayLocale], { type: "language" }).of(languageCode) ?? languageCode;
  return languageName.charAt(0).toLocaleUpperCase(displayLocale) + languageName.slice(1);
}

export function resolveLocalizedContent(preferredLocale: string): LocalizedContent {
  const usesPortuguese = preferredLocale.toLowerCase().startsWith("pt");
  return {
    locale: usesPortuguese ? "pt-BR" : preferredLocale,
    messages: usesPortuguese ? portugueseMessages : englishMessages,
    translateRaceName: (officialRaceName) =>
      usesPortuguese ? (portugueseRaceNames[officialRaceName] ?? officialRaceName) : officialRaceName.replace("Grand Prix", "GP"),
  };
}
