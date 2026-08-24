import { describe, it, expect } from 'vitest';
import {
  formatTimestamp,
  formatFullDate,
  formatPercent,
  formatLatency,
  formatCurrency,
  formatScore,
} from '../utils/formatters';

describe('SentinelMesh UI Formatters', () => {
  describe('formatTimestamp', () => {
    it('formats ISO-8601 string correctly', () => {
      const result = formatTimestamp('2026-08-24T12:00:00.123Z');
      expect(result).not.toContain('Invalid Date');
      expect(result).not.toBe('Time unavailable');
      expect(result).toMatch(/^\d{2}:\d{2}:\d{2}\.\d{3}$/);
    });

    it('formats RFC3339 string correctly', () => {
      const result = formatTimestamp('2026-08-24T12:00:00+05:30');
      expect(result).not.toContain('Invalid Date');
      expect(result).not.toBe('Time unavailable');
    });

    it('formats Date instance correctly', () => {
      const date = new Date(1700000000000);
      const result = formatTimestamp(date);
      expect(result).not.toContain('Invalid Date');
    });

    it('formats Unix epoch in seconds correctly', () => {
      const result = formatTimestamp(1700000000);
      expect(result).not.toContain('Invalid Date');
    });

    it('formats Unix epoch in milliseconds correctly', () => {
      const result = formatTimestamp(1700000000000);
      expect(result).not.toContain('Invalid Date');
    });

    it('returns "Time unavailable" for undefined, null, or empty string', () => {
      expect(formatTimestamp(undefined)).toBe('Time unavailable');
      expect(formatTimestamp(null)).toBe('Time unavailable');
      expect(formatTimestamp('')).toBe('Time unavailable');
    });

    it('returns "Time unavailable" for invalid date strings (never "Invalid Date")', () => {
      const result = formatTimestamp('invalid-date-string');
      expect(result).toBe('Time unavailable');
      expect(result).not.toContain('Invalid Date');
    });
  });

  describe('numeric formatters', () => {
    it('formats percentages correctly without floating noise', () => {
      expect(formatPercent(0.934889)).toBe('93.5%');
      expect(formatPercent(0.42)).toBe('42.0%');
      expect(formatPercent(95.4)).toBe('95.4%');
      expect(formatPercent(undefined)).toBe('0.0%');
    });

    it('formats latency correctly', () => {
      expect(formatLatency(421.34)).toBe('421ms');
      expect(formatLatency(0)).toBe('0ms');
      expect(formatLatency(null)).toBe('0ms');
    });

    it('formats currency correctly', () => {
      expect(formatCurrency(0.0042)).toBe('$0.0042');
      expect(formatCurrency(0.123)).toBe('$0.12');
      expect(formatCurrency(0)).toBe('$0.0000');
    });

    it('formats score / UCB correctly', () => {
      expect(formatScore(0.729124)).toBe('0.729');
      expect(formatScore(0.59)).toBe('0.590');
    });
  });
});
