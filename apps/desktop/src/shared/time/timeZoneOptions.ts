// Turns raw IANA ids ("America/Sao_Paulo") into options people can recognize and search:
// country, city, the readable zone name ("Horário Padrão de Brasília") and the GMT offset.
import { countryCodeByTimeZone } from "./timeZoneCountries.generated";

export type TimeZoneOption = {
  timeZoneId: string;
  /** ISO 3166 code from the tz database; null for zones outside any country. */
  countryCode: string | null;
  /** Country name in the interface language ("Brasil", "Estados Unidos"). */
  countryName: string;
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

// Intl.DisplayNames is costly to build: one per locale for the ~400 zones.
const countryNamesByLocale = new Map<string, Intl.DisplayNames>();

function countryNameFor(countryCode: string, locale: string): string {
  let countryNames = countryNamesByLocale.get(locale);
  if (!countryNames) {
    countryNames = new Intl.DisplayNames([locale], { type: "region" });
    countryNamesByLocale.set(locale, countryNames);
  }
  return countryNames.of(countryCode) ?? countryCode;
}

export function describeTimeZoneOption(timeZoneId: string, locale: string, referenceInstant = new Date()): TimeZoneOption {
  const offsetMinutes = parseOffsetMinutes(readTimeZoneName(timeZoneId, locale, referenceInstant, "shortOffset"));
  const regionKey = timeZoneId.split("/")[0] ?? "";
  const regionNames = regionNamesByLocale[locale.slice(0, 2)] ?? regionNamesByLocale.en!;
  const cityName = cityNameFromZoneId(timeZoneId);
  const regionName = regionNames[regionKey] ?? regionKey;
  const countryCode = countryCodeByTimeZone[timeZoneId] ?? null;
  // Zones outside any country (none in practice today) fall back to their region.
  const countryName = countryCode ? countryNameFor(countryCode, locale) : regionName;
  const genericName = readTimeZoneName(timeZoneId, locale, referenceInstant, "longGeneric");
  const offsetLabel = formatOffsetLabel(offsetMinutes);

  // Offsets are searchable as "-3", "gmt-3" or "utc-3".
  const plainOffset = offsetLabel.replace("GMT", "").replace("−", "-");
  const offsetVariants = plainOffset ? `${plainOffset} gmt${plainOffset} utc${plainOffset}` : "gmt utc +0 gmt+0 utc+0";
  return {
    timeZoneId,
    countryCode,
    countryName,
    cityName,
    regionName,
    genericName,
    offsetMinutes,
    offsetLabel,
    searchText: normalizeForSearch(`${countryName} ${cityName} ${timeZoneId} ${regionName} ${genericName} ${offsetVariants}`),
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

export type TimeZoneCountryGroup = { countryName: string; countryCode: string | null; timeZoneOptions: TimeZoneOption[] };

/**
 * One group per country, alphabetical in the interface language, with `firstCountryCode`
 * (the computer's country) on top. Inside a country: west to east, then by city.
 */
export function groupByCountry(timeZoneOptions: TimeZoneOption[], locale: string, firstCountryCode: string | null = null): TimeZoneCountryGroup[] {
  const groupsByCountryName = new Map<string, TimeZoneCountryGroup>();
  for (const timeZoneOption of timeZoneOptions) {
    const countryGroup = groupsByCountryName.get(timeZoneOption.countryName) ?? {
      countryName: timeZoneOption.countryName,
      countryCode: timeZoneOption.countryCode,
      timeZoneOptions: [],
    };
    countryGroup.timeZoneOptions.push(timeZoneOption);
    groupsByCountryName.set(timeZoneOption.countryName, countryGroup);
  }
  const countryGroups = [...groupsByCountryName.values()];
  for (const countryGroup of countryGroups) {
    countryGroup.timeZoneOptions.sort(
      (firstOption, secondOption) => firstOption.offsetMinutes - secondOption.offsetMinutes || firstOption.cityName.localeCompare(secondOption.cityName, locale),
    );
  }
  return countryGroups.sort((firstGroup, secondGroup) => {
    if (firstCountryCode && firstGroup.countryCode !== secondGroup.countryCode) {
      if (firstGroup.countryCode === firstCountryCode) return -1;
      if (secondGroup.countryCode === firstCountryCode) return 1;
    }
    return firstGroup.countryName.localeCompare(secondGroup.countryName, locale);
  });
}
