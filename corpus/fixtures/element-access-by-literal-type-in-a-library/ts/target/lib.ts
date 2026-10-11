interface Prefs {
  lang: string;
  zone: string;
}
const prefs: Prefs = { lang: "en", zone: "utc" };

export class PrefReader {
  read(key: keyof Prefs): string {
    return prefs[key];
  }
}
