import { describe, it, expect } from 'vitest';
import ja from './translations/ja.json';
import en from './translations/en.json';
import de from './translations/de.json';
import es from './translations/es.json';
import fr from './translations/fr.json';
import itJson from './translations/it.json';
import pt from './translations/pt.json';
import ru from './translations/ru.json';
import zh from './translations/zh.json';

type Node = string | { [key: string]: Node };

const flatten = (node: Node, prefix = ''): Record<string, string> => {
  if (typeof node === 'string') return { [prefix]: node };
  return Object.entries(node).reduce<Record<string, string>>((acc, [key, value]) => {
    return { ...acc, ...flatten(value, prefix ? `${prefix}.${key}` : key) };
  }, {});
};

const locales: Record<string, Node> = { en, de, es, fr, it: itJson, pt, ru, zh };
const jaKeys = Object.keys(flatten(ja));

describe('translation key parity', () => {
  it.each(Object.keys(locales))('%s.json has exactly the same keys as ja.json', (locale) => {
    const keys = Object.keys(flatten(locales[locale]));
    expect(keys.sort()).toEqual(jaKeys.sort());
  });

  it.each(Object.keys(locales))('%s.json preserves {{placeholders}} of ja.json', (locale) => {
    const jaFlat = flatten(ja);
    const flat = flatten(locales[locale]);
    const placeholders = (s: string) => s.match(/\{\{\w+\}\}/g)?.sort() ?? [];
    for (const key of jaKeys) {
      expect(placeholders(flat[key]), `${locale}:${key}`).toEqual(placeholders(jaFlat[key]));
    }
  });

  it('no value is an empty string in any locale', () => {
    for (const locale of Object.keys(locales)) {
      for (const [key, value] of Object.entries(flatten(locales[locale]))) {
        expect(value.length > 0, `${locale}:${key}`).toBe(true);
      }
    }
  });
});
