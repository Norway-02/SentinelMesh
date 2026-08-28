/**
 * SentinelMesh UI Formatting Utilities
 * Standardized date, time, and numeric formatting across all Control Plane views.
 */

/**
 * Formats a timestamp into human-readable local time with fallback for missing/invalid data.
 * Supports ISO-8601 strings, RFC3339 strings, Date objects, Unix seconds, and Unix milliseconds.
 * Never returns "Invalid Date".
 */
export function formatTimestamp(ts?: string | number | Date | null): string {
  if (ts === undefined || ts === null || ts === '') {
    return 'Time unavailable';
  }

  try {
    let date: Date;

    if (ts instanceof Date) {
      date = ts;
    } else if (typeof ts === 'number') {
      // If ts is in seconds (10 digits, e.g. 1700000000), convert to ms
      const ms = ts < 1e11 ? ts * 1000 : ts;
      date = new Date(ms);
    } else if (typeof ts === 'string') {
      // Numeric string epoch check
      const num = Number(ts);
      if (!isNaN(num) && ts.trim() !== '') {
        const ms = num < 1e11 ? num * 1000 : num;
        date = new Date(ms);
      } else {
        date = new Date(ts);
      }
    } else {
      return 'Time unavailable';
    }

    if (isNaN(date.getTime())) {
      return 'Time unavailable';
    }

    return date.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }) + '.' + String(date.getMilliseconds()).padStart(3, '0');
  } catch {
    return 'Time unavailable';
  }
}

/**
 * Formats ISO date string to short date & time (e.g., "Aug 24, 12:15:30")
 */
export function formatFullDate(ts?: string | number | Date | null): string {
  if (!ts) return 'Time unavailable';
  try {
    const d = new Date(ts);
    if (isNaN(d.getTime())) return 'Time unavailable';
    return d.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  } catch {
    return 'Time unavailable';
  }
}

/**
 * Formats a decimal ratio or raw score into a percentage (e.g. 0.9548 -> "95.5%")
 */
export function formatPercent(value?: number | null, decimals = 1): string {
  if (value === undefined || value === null || isNaN(value)) {
    return '0.0%';
  }
  // If value is already in range 0-100 (e.g., 95.4), don't multiply by 100
  const pct = value <= 1.0 && value >= 0 ? value * 100 : value;
  return `${pct.toFixed(decimals)}%`;
}

/**
 * Formats latency in milliseconds or seconds (e.g. 421.34 -> "421ms", 92241.6 -> "92.24s")
 * Prevents unformatted float artifacts.
 */
export function formatLatency(ms?: number | null): string {
  if (ms === undefined || ms === null || isNaN(ms)) {
    return '0ms';
  }
  // Convert nanoseconds to milliseconds if needed
  const val = ms > 100000 ? ms / 1000000 : ms;
  if (val >= 10000) {
    return `${(val / 1000).toFixed(2)}s`;
  }
  if (val >= 1000) {
    return `${Math.round(val).toLocaleString()}ms`;
  }
  if (val < 1 && val > 0) {
    return `${val.toFixed(1)}ms`;
  }
  return `${Math.round(val)}ms`;
}

/**
 * Formats currency USD with appropriate precision (e.g. 0.0042 -> "$0.0042")
 */
export function formatCurrency(usd?: number | null): string {
  if (usd === undefined || usd === null || isNaN(usd)) {
    return '$0.0000';
  }
  if (usd < 0.01) {
    return `$${usd.toFixed(4)}`;
  }
  return `$${usd.toFixed(2)}`;
}

/**
 * Formats a score/UCB value to significant digits (e.g. 0.729124 -> "0.729")
 */
export function formatScore(score?: number | null, digits = 3): string {
  if (score === undefined || score === null || isNaN(score)) {
    return '0.000';
  }
  return score.toFixed(digits);
}
