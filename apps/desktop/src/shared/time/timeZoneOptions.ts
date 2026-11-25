// Turns raw IANA ids ("America/Sao_Paulo") into options people can recognize and search:
// city, region, the readable zone name ("Horário Padrão de Brasília") and the GMT offset.

export type TimeZoneOption = {
  timeZoneId: string;
  cityName: string;
  regionName: string;
  genericName: string;
  offsetMinutes: number;
  offsetLabel: string;
  /** Normalized text matched by the search box. */
  searchText: string;
};

// IANA ids are ASCII: restore the native spelling of common cities.
const cityNameFixes: Record<string, string> = {
  Sao_Paulo: "São Paulo",
  Belem: "Belém",
  Cuiaba: "Cuiabá",
  Maceio: "Maceió",
  Araguaina: "Araguaína",
  Santarem: "Santarém",
  Bogota: "Bogotá",
  Asuncion: "Asunción",
  Cancun: "Cancún",
  Merida: "Mérida",
  Curacao: "Curaçao",
  Reykjavik: "Reykjavík",
  Mexico_City: "Mexico City",
};

const regionNamesByLocale: Record<string, Record<string, string>> = {
  pt: {
    Africa: "África",
    America: "Américas",
    Antarctica: "Antártida",
    Arctic: "Ártico",
    Asia: "Ásia",
    Atlantic: "Atlântico",
    Australia: "Austrália",
    Europe: "Europa",
    Indian: "Oceano Índico",
    Pacific: "Pacífico",
  },
  en: {
    Africa: "Africa",
    America: "Americas",
    Antarctica: "Antarctica",
    Arctic: "Arctic",
    Asia: "Asia",
    Atlantic: "Atlantic",
    Australia: "Australia",
    Europe: "Europe",
    Indian: "Indian Ocean",
    Pacific: "Pacific",
  },
};

export function normalizeForSearch(searchText: string): string {
  return searchText
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[_−]/g, (matchedCharacter) => (matchedCharacter === "_" ? " " : "-"))
    .toLowerCase()
    .trim();
}

/** "America/Argentina/Buenos_Aires" → "Buenos Aires"; "America/Sao_Paulo" → "São Paulo". */
export function cityNameFromZoneId(timeZoneId: string): string {
  const lastSegment = timeZoneId.split("/").at(-1) ?? timeZoneId;
  return cityNameFixes[lastSegment] ?? lastSegment.replace(/_/g, " ");
}

/** "GMT-3" → −180, "GMT+5:30" → 330, "GMT" → 0. */
export function parseOffsetMinutes(shortOffset: string): number {
  const offsetMatch = /GMT([+-])(\d{1,2})(?::(\d{2}))?/.exec(shortOffset);
  if (!offsetMatch) return 0;
  const offsetSign = offsetMatch[1] === "-" ? -1 : 1;
  return offsetSign * (Number(offsetMatch[2]) * 60 + Number(offsetMatch[3] ?? 0));
}

/** −180 → "GMT−3", 330 → "GMT+5:30", 0 → "GMT" (true minus sign for display). */
export function formatOffsetLabel(offsetMinutes: number): string {
  if (offsetMinutes === 0) return "GMT";
  const absoluteMinutes = Math.abs(offsetMinutes);
  const offsetHours = Math.floor(absoluteMinutes / 60);
  const remainingMinutes = absoluteMinutes % 60;
  const signCharacter = offsetMinutes < 0 ? "−" : "+";
  return `GMT${signCharacter}${offsetHours}${remainingMinutes ? `:${String(remainingMinutes).padStart(2, "0")}` : ""}`;
}

function readTimeZoneName(timeZoneId: string, locale: string, referenceInstant: Date, nameStyle: Intl.DateTimeFormatOptions["timeZoneName"]): string {
  return (
    new Intl.DateTimeFormat(locale, { timeZone: timeZoneId, timeZoneName: nameStyle })
      .formatToParts(referenceInstant)
      .find((datePart) => datePart.type === "timeZoneName")?.value ?? ""
  );
}

export function describeTimeZoneOption(timeZoneId: string, locale: string, referenceInstant = new Date()): TimeZoneOption {
  const offsetMinutes = parseOffsetMinutes(readTimeZoneName(timeZoneId, locale, referenceInstant, "shortOffset"));
  const regionKey = timeZoneId.split("/")[0] ?? "";
  const regionNames = regionNamesByLocale[locale.slice(0, 2)] ?? regionNamesByLocale.en!;
  const cityName = cityNameFromZoneId(timeZoneId);
  const regionName = regionNames[regionKey] ?? regionKey;
  const genericName = readTimeZoneName(timeZoneId, locale, referenceInstant, "longGeneric");
  const offsetLabel = formatOffsetLabel(offsetMinutes);

  // Offsets are searchable as "-3", "gmt-3" or "utc-3".
  const plainOffset = offsetLabel.replace("GMT", "").replace("−", "-");
  const offsetVariants = plainOffset ? `${plainOffset} gmt${plainOffset} utc${plainOffset}` : "gmt utc +0 gmt+0 utc+0";
  return {
    timeZoneId,
    cityName,
    regionName,
    genericName,
    offsetMinutes,
    offsetLabel,
    searchText: normalizeForSearch(`${cityName} ${timeZoneId} ${regionName} ${genericName} ${offsetVariants}`),
  };
}

/** Every zone, west to east, then by city name. */
export function buildTimeZoneOptions(timeZoneIds: string[], locale: string, referenceInstant = new Date()): TimeZoneOption[] {
  return timeZoneIds
    .map((timeZoneId) => describeTimeZoneOption(timeZoneId, locale, referenceInstant))
    .sort((firstOption, secondOption) => firstOption.offsetMinutes - secondOption.offsetMinutes || firstOption.cityName.localeCompare(secondOption.cityName, locale));
}

/** Every word typed must appear (in any order): "europa lisboa", "brasilia", "-3". */
export function filterTimeZoneOptions(timeZoneOptions: TimeZoneOption[], searchQuery: string): TimeZoneOption[] {
  const queryWords = normalizeForSearch(searchQuery).split(/\s+/).filter(Boolean);
  if (queryWords.length === 0) return timeZoneOptions;
  return timeZoneOptions.filter((timeZoneOption) => queryWords.every((queryWord) => timeZoneOption.searchText.includes(queryWord)));
}

export type TimeZoneOffsetGroup = { offsetLabel: string; timeZoneOptions: TimeZoneOption[] };

/** Consecutive options with the same offset, for the "GMT−3" headings of the list. */
export function groupByOffset(timeZoneOptions: TimeZoneOption[]): TimeZoneOffsetGroup[] {
  const offsetGroups: TimeZoneOffsetGroup[] = [];
  for (const timeZoneOption of timeZoneOptions) {
    const lastGroup = offsetGroups.at(-1);
    if (lastGroup && lastGroup.offsetLabel === timeZoneOption.offsetLabel) lastGroup.timeZoneOptions.push(timeZoneOption);
    else offsetGroups.push({ offsetLabel: timeZoneOption.offsetLabel, timeZoneOptions: [timeZoneOption] });
  }
  return offsetGroups;
}
