import { describe, it, expect } from 'vitest';
import { isValidHost, isValidURL, isValidGoDuration, isValidHHMM, normalizeDays, areValidDays } from './validation';

describe('isValidHost', () => {
  it('returns true for "localhost"', () => {
    expect(isValidHost('localhost')).toBe(true);
  });

  it('returns true for valid domains', () => {
    expect(isValidHost('example.com')).toBe(true);
    expect(isValidHost('sub.example.com')).toBe(true);
    expect(isValidHost('example.co.uk')).toBe(true);
    expect(isValidHost('a.b')).toBe(true);
    expect(isValidHost('127.0.0.1')).toBe(true);
    expect(isValidHost('server-01.example.com')).toBe(true);
    expect(isValidHost('example-domain.com')).toBe(true);
    expect(isValidHost('a'.repeat(63) + '.com')).toBe(true); // max label length
  });

  it('returns true for hosts with leading/trailing whitespace', () => {
    expect(isValidHost('  example.com  ')).toBe(true);
  });

  it('returns false for empty string', () => {
    expect(isValidHost('')).toBe(false);
  });

  it('returns false for whitespace-only string', () => {
    expect(isValidHost('   ')).toBe(false);
  });

  it('returns false for hosts with spaces', () => {
    expect(isValidHost('example .com')).toBe(false);
    expect(isValidHost('exam ple.com')).toBe(false);
  });

  it('returns false for hosts with slashes', () => {
    expect(isValidHost('example.com/')).toBe(false);
    expect(isValidHost('example.com/path')).toBe(false);
    expect(isValidHost('/example.com')).toBe(false);
  });

  it('returns false for single label (no dots)', () => {
    expect(isValidHost('localhost2')).toBe(false);
    expect(isValidHost('example')).toBe(false);
  });

  it('returns false for labels starting or ending with hyphen', () => {
    expect(isValidHost('-example.com')).toBe(false);
    expect(isValidHost('example-.com')).toBe(false);
    expect(isValidHost('-example-.com')).toBe(false);
  });

  it('returns false for empty labels (consecutive dots)', () => {
    expect(isValidHost('example..com')).toBe(false);
  });

  it('returns false for labels exceeding 63 characters', () => {
    expect(isValidHost('a'.repeat(64) + '.com')).toBe(false);
  });
});

describe('isValidURL', () => {
  it('returns true for http URLs', () => {
    expect(isValidURL('http://example.com')).toBe(true);
    expect(isValidURL('http://localhost:8080')).toBe(true);
    expect(isValidURL('http://127.0.0.1')).toBe(true);
    expect(isValidURL('http://example.com/path?query=1')).toBe(true);
  });

  it('returns true for https URLs', () => {
    expect(isValidURL('https://example.com')).toBe(true);
    expect(isValidURL('https://localhost:8080')).toBe(true);
    expect(isValidURL('https://example.com/path')).toBe(true);
    expect(isValidURL('https://127.0.0.1')).toBe(true);
  });

  it('returns false for non-http protocols', () => {
    expect(isValidURL('ftp://example.com')).toBe(false);
    expect(isValidURL('ws://example.com')).toBe(false);
    expect(isValidURL('file:///etc/passwd')).toBe(false);
    expect(isValidURL('javascript:alert(1)')).toBe(false);
  });

  it('returns false for malformed URLs', () => {
    expect(isValidURL('not-a-url')).toBe(false);
    expect(isValidURL('://example.com')).toBe(false);
    expect(isValidURL('')).toBe(false);
    expect(isValidURL('http:')).toBe(false);
  });

  it('returns false for URLs without protocol', () => {
    expect(isValidURL('example.com')).toBe(false);
    expect(isValidURL('www.example.com')).toBe(false);
  });
});

describe('isValidGoDuration', () => {
  it('returns true for valid Go durations', () => {
    expect(isValidGoDuration('500ms')).toBe(true);
    expect(isValidGoDuration('2s')).toBe(true);
    expect(isValidGoDuration('1m30s')).toBe(true);
    expect(isValidGoDuration('1h2m3s')).toBe(true);
    expect(isValidGoDuration('250us')).toBe(true);
    expect(isValidGoDuration('10ns')).toBe(true);
    expect(isValidGoDuration('100µs')).toBe(true);
    expect(isValidGoDuration('1h')).toBe(true);
    expect(isValidGoDuration('1m')).toBe(true);
    expect(isValidGoDuration('1s')).toBe(true);
    expect(isValidGoDuration('1ms')).toBe(true);
    expect(isValidGoDuration('1us')).toBe(true);
    expect(isValidGoDuration('1ns')).toBe(true);
  });

  it('returns true for durations with whitespace', () => {
    expect(isValidGoDuration('  500ms  ')).toBe(true);
  });

  it('returns true for compound durations', () => {
    expect(isValidGoDuration('2h30m45s')).toBe(true);
    expect(isValidGoDuration('1h0m0s')).toBe(true);
  });

  it('returns false for empty string', () => {
    expect(isValidGoDuration('')).toBe(false);
  });

  it('returns false for invalid units', () => {
    expect(isValidGoDuration('500x')).toBe(false);
    expect(isValidGoDuration('1sec')).toBe(false);
    expect(isValidGoDuration('1minute')).toBe(false);
    expect(isValidGoDuration('1hour')).toBe(false);
  });

  it('returns false for missing number or unit', () => {
    expect(isValidGoDuration('ms')).toBe(false);
    expect(isValidGoDuration('500')).toBe(false);
    expect(isValidGoDuration('s')).toBe(false);
  });

  it('returns false for negative durations', () => {
    expect(isValidGoDuration('-500ms')).toBe(false);
    expect(isValidGoDuration('-1s')).toBe(false);
  });

  it('returns false for decimal numbers', () => {
    expect(isValidGoDuration('1.5s')).toBe(false);
    expect(isValidGoDuration('0.5m')).toBe(false);
  });
});

describe('isValidHHMM', () => {
  it('returns true for valid 24h time formats', () => {
    expect(isValidHHMM('00:00')).toBe(true);
    expect(isValidHHMM('09:30')).toBe(true);
    expect(isValidHHMM('12:00')).toBe(true);
    expect(isValidHHMM('18:45')).toBe(true);
    expect(isValidHHMM('23:59')).toBe(true);
    expect(isValidHHMM('01:01')).toBe(true);
    expect(isValidHHMM('10:10')).toBe(true);
  });

  it('returns true for time with whitespace', () => {
    expect(isValidHHMM('  09:30  ')).toBe(true);
  });

  it('returns false for hours >= 24', () => {
    expect(isValidHHMM('24:00')).toBe(false);
    expect(isValidHHMM('25:00')).toBe(false);
    expect(isValidHHMM('99:99')).toBe(false);
  });

  it('returns false for minutes >= 60', () => {
    expect(isValidHHMM('12:60')).toBe(false);
    expect(isValidHHMM('12:99')).toBe(false);
  });

  it('returns false for single-digit hour or minute', () => {
    expect(isValidHHMM('9:30')).toBe(false);
    expect(isValidHHMM('09:5')).toBe(false);
  });

  it('returns false for empty string', () => {
    expect(isValidHHMM('')).toBe(false);
  });

  it('returns false for malformed time', () => {
    expect(isValidHHMM('12-30')).toBe(false);
    expect(isValidHHMM('1230')).toBe(false);
    expect(isValidHHMM('12:30:00')).toBe(false);
    expect(isValidHHMM('abc')).toBe(false);
  });

  it('returns false for negative hours or minutes', () => {
    expect(isValidHHMM('-1:30')).toBe(false);
    expect(isValidHHMM('12:-30')).toBe(false);
  });
});

describe('normalizeDays', () => {
  it('splits comma-separated string', () => {
    expect(normalizeDays('mon,tue,wed')).toEqual(['mon', 'tue', 'wed']);
  });

  it('trims whitespace around entries', () => {
    expect(normalizeDays('mon, tue , wed')).toEqual(['mon', 'tue', 'wed']);
    expect(normalizeDays(' mon ,tue')).toEqual(['mon', 'tue']);
  });

  it('lowercases all entries', () => {
    expect(normalizeDays('Mon,TUE,Wed')).toEqual(['mon', 'tue', 'wed']);
    expect(normalizeDays('MON')).toEqual(['mon']);
  });

  it('filters empty entries', () => {
    expect(normalizeDays('mon,,wed')).toEqual(['mon', 'wed']);
    expect(normalizeDays('mon, ,wed')).toEqual(['mon', 'wed']);
  });

  it('returns empty array for empty string', () => {
    expect(normalizeDays('')).toEqual([]);
  });

  it('returns empty array for string with only commas', () => {
    expect(normalizeDays(',,,')).toEqual([]);
  });

  it('returns empty array for string with only whitespace', () => {
    expect(normalizeDays('   ')).toEqual([]);
  });

  it('handles single day', () => {
    expect(normalizeDays('mon')).toEqual(['mon']);
  });
});

describe('areValidDays', () => {
  it('returns true for valid 3-letter day names', () => {
    expect(areValidDays(['mon'])).toBe(true);
    expect(areValidDays(['tue'])).toBe(true);
    expect(areValidDays(['wed'])).toBe(true);
    expect(areValidDays(['thu'])).toBe(true);
    expect(areValidDays(['fri'])).toBe(true);
    expect(areValidDays(['sat'])).toBe(true);
    expect(areValidDays(['sun'])).toBe(true);
  });

  it('returns true for all days of the week', () => {
    expect(areValidDays(['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'])).toBe(true);
  });

  it('returns true for subset of valid days', () => {
    expect(areValidDays(['mon', 'fri'])).toBe(true);
    expect(areValidDays(['sat', 'sun'])).toBe(true);
  });

  it('returns false for invalid day names', () => {
    expect(areValidDays(['monday'])).toBe(false);
    expect(areValidDays(['tues'])).toBe(false);
    expect(areValidDays(['xyz'])).toBe(false);
    expect(areValidDays(['Mon'])).toBe(false); // case-sensitive
    expect(areValidDays(['MON'])).toBe(false);
  });

  it('returns false for mixed valid and invalid days', () => {
    expect(areValidDays(['mon', 'invalid'])).toBe(false);
    expect(areValidDays(['mon', 'tuesday'])).toBe(false);
  });

  it('returns true for empty array', () => {
    expect(areValidDays([])).toBe(true); // every() returns true for empty arrays
  });

  it('returns false for non-lowercase day names', () => {
    expect(areValidDays(['Tue'])).toBe(false);
    expect(areValidDays(['WED'])).toBe(false);
  });
});
