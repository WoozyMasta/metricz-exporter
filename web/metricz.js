/**
 * MetricZ public status client + lightweight canvas time series chart.
 *
 * Expected API shape (per instance or per all instances):
 * {
 *   values: { [metricName]: number },
 *   labels: { [metricName]: { [labelKey]: string[] } }
 * }
 *
 * When requesting all instances:
 * {
 *   [instanceId]: { values, labels }
 * }
 */

/**
 * Safe JSON fetch with basic HTTP error handling.
 * @param {string} url
 * @param {AbortSignal=} signal
 * @returns {Promise<any>}
 */
async function fetchJSON(url, signal) {
  const res = await fetch(url, {
    signal,
    cache: "no-store"
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    const err = new Error(`HTTP ${res.status} ${res.statusText}: ${text}`.trim());
    err.status = res.status;
    throw err;
  }
  return res.json();
}

/**
 * MetricZClient polls /api/v1/status endpoints and emits normalized data.
 *
 * - If serverIds is empty: GET {baseUrl} (all instances)
 * - Else: GET {baseUrl}/{id} for each instance in parallel
 *
 * It pauses polling when the tab is hidden and resumes with anti-spam logic.
 */
export class MetricZClient {
  /**
   * @param {{
   *  baseUrl?: string,
   *  serverIds?: string[],
   *  interval?: number, // seconds
   *  onUpdate?: (data: any) => void,
   *  onError?: (err: any) => void
   * }} options
   */
  constructor(options = {}) {
    this.baseUrl = options.baseUrl || "/api/v1/status";
    this.serverIds = Array.isArray(options.serverIds) ? options.serverIds : [];
    this.defaultIntervalMs = (options.interval || 15) * 1000;
    this.currentIntervalMs = this.defaultIntervalMs;

    this.timer = null;
    this.lastFetchTime = 0;
    this.isTabActive = true;

    this.onUpdate = typeof options.onUpdate === "function" ? options.onUpdate : () => {};
    this.onError = typeof options.onError === "function" ? options.onError : () => {};

    this._abort = null;
    this._inFlight = false;

    this._initVisibilityHandler();
  }

  /** Start polling. */
  start() {
    if (!this.timer && !this._inFlight) this._fetchData();
  }

  /** Stop polling and abort any in-flight request. */
  stop() {
    clearTimeout(this.timer);
    this.timer = null;
    this._abortFetch();
  }

  _abortFetch() {
    if (this._abort) {
      this._abort.abort();
      this._abort = null;
    }
  }

  _scheduleNext(delayMs) {
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this._fetchData(), delayMs);
  }

  /**
   * Fetch data once and reschedule.
   * Output is always normalized to: { [instanceId]: { values, labels } }
   */
  async _fetchData() {
    if (this._inFlight) return;
    this._inFlight = true;

    this._abortFetch();
    this._abort = new AbortController();

    try {
      let data;

      if (this.serverIds.length > 0) {
        // Fetch specific instances in parallel.
        const jobs = this.serverIds.map(async (id) => {
          const d = await fetchJSON(`${this.baseUrl}/${encodeURIComponent(id)}`, this._abort.signal);
          return [id, d];
        });

        const results = await Promise.all(jobs);
        data = Object.fromEntries(results);
      } else {
        // Fetch all instances.
        data = await fetchJSON(this.baseUrl, this._abort.signal);
      }

      this.lastFetchTime = Date.now();

      // Optionally adjust interval if the scrape interval metric is present.
      this._adjustInterval(data);

      // Emit.
      this.onUpdate(data);
    } catch (err) {
      // Ignore abort errors (stop() or tab hide can abort).
      if (err && err.name === "AbortError") return;
      this.onError(err);
    } finally {
      this._inFlight = false;

      // Schedule next run only when tab is active.
      if (this.isTabActive) this._scheduleNext(this.currentIntervalMs);
    }
  }

  /**
   * Dynamically updates polling interval based on any instance metric:
   * values.dayz_metricz_scrape_interval_seconds (seconds).
   * @param {Record<string, any>} data
   */
  _adjustInterval(data) {
    let newIntervalMs = null;

    for (const server of Object.values(data)) {
      const v = server?.values?.dayz_metricz_scrape_interval_seconds;
      if (typeof v === "number" && Number.isFinite(v) && v > 0) {
        newIntervalMs = Math.max(1000, Math.floor(v * 1000));
        break;
      }
    }

    if (newIntervalMs && newIntervalMs !== this.currentIntervalMs) {
      this.currentIntervalMs = newIntervalMs;
    }
  }

  /**
   * Visibility handling:
   * - When hidden: stop timers and abort fetch to avoid background traffic.
   * - When visible: fetch immediately if overdue, else wait remaining interval.
   */
  _initVisibilityHandler() {
    document.addEventListener("visibilitychange", () => {
      if (document.hidden) {
        this.isTabActive = false;
        clearTimeout(this.timer);
        this.timer = null;
        this._abortFetch();
        return;
      }

      this.isTabActive = true;
      const now = Date.now();
      const elapsed = now - this.lastFetchTime;

      if (elapsed >= this.currentIntervalMs) {
        this._fetchData();
      } else {
        this._scheduleNext(this.currentIntervalMs - elapsed);
      }
    });
  }

  /**
   * Helper: get the first label value for a given instance/metric/labelKey.
   * New API format returns label values as arrays; this returns the first item.
   * @param {any} instanceData
   * @param {string} metricName
   * @param {string} labelKey
   * @returns {string|null}
   */
  static getLabelFirst(instanceData, metricName, labelKey) {
    const arr = instanceData?.labels?.[metricName]?.[labelKey];
    return Array.isArray(arr) && arr.length > 0 ? String(arr[0]) : null;
  }

  /**
   * Helper: get all label values for a given instance/metric/labelKey.
   * @param {any} instanceData
   * @param {string} metricName
   * @param {string} labelKey
   * @returns {string[]}
   */
  static getLabelAll(instanceData, metricName, labelKey) {
    const arr = instanceData?.labels?.[metricName]?.[labelKey];
    return Array.isArray(arr) ? arr.map(String) : [];
  }
}

/**
 * TimeseriesChart draws a simple line chart with area fill on an HTML canvas.
 * Optimized for small dashboards: no dependencies, no layouts, minimal allocations.
 */
export class TimeseriesChart {
  /**
   * @param {HTMLCanvasElement} canvasElement
   * @param {{
   *  maxPoints?: number,
   *  colors?: { high?: string, medium?: string, low?: string },
   *  thresholds?: { medium?: number, low?: number }
   * }} options
   */
  constructor(canvasElement, options = {}) {
    this.canvas = canvasElement;
    this.ctx = this.canvas.getContext("2d");
    this.data = [];
    this.maxPoints = options.maxPoints || 40;

    // Color rules (HEX strings). Keep it configurable for embedding.
    this.colors = {
      high: options.colors?.high || "#5cb85c", // > medium threshold
      medium: options.colors?.medium || "#f0ad4e", // low..medium
      low: options.colors?.low || "#d9534f" // < low threshold
    };

    // Thresholds for selecting the current line color.
    this.thresholds = {
      medium: options.thresholds?.medium ?? 40,
      low: options.thresholds?.low ?? 20
    };

    // First update can optionally backfill history around the initial value.
    this.isInitialized = false;

    // HiDPI canvas setup.
    const dpr = window.devicePixelRatio || 1;
    const rect = this.canvas.getBoundingClientRect();
    this.canvas.width = Math.floor(rect.width * dpr);
    this.canvas.height = Math.floor(rect.height * dpr);
    this.ctx.scale(dpr, dpr);

    this.width = rect.width;
    this.height = rect.height;
  }

  /**
   * Push a new numeric sample.
   * @param {number} value
   */
  push(value) {
    const v = Number(value);
    if (!Number.isFinite(v)) return;

    if (!this.isInitialized) {
      this._generateInitialHistory(v);
      this.isInitialized = true;
    }

    this.data.push(v);
    if (this.data.length > this.maxPoints) this.data.shift();
    this.draw();
  }

  /**
   * Fill the chart with synthetic points around the first value.
   * This avoids an "empty chart" look on first render.
   * @param {number} baseValue
   */
  _generateInitialHistory(baseValue) {
    const points = this.maxPoints - 1;
    const variation = Math.max(5, baseValue * 0.03);

    for (let i = 0; i < points; i++) {
      const noise = (Math.random() - 0.5) * 2 * variation;
      const val = Math.max(0, baseValue + noise);
      this.data.push(val);
    }
  }

  _getCurrentColor() {
    const current = this.data[this.data.length - 1] ?? 0;
    if (current < this.thresholds.low) return this.colors.low;
    if (current < this.thresholds.medium) return this.colors.medium;
    return this.colors.high;
  }

  draw() {
    const {
      ctx,
      width,
      height,
      data
    } = this;
    ctx.clearRect(0, 0, width, height);
    if (data.length === 0) return;

    const currentColor = this._getCurrentColor();

    // Scale Y: include a soft ceiling to avoid flatlining at typical FPS caps.
    const maxVal = Math.max(60, ...data) * 1.1;
    const minVal = 0;
    const range = Math.max(1, maxVal - minVal);

    const getX = (i) => (i / (this.maxPoints - 1)) * width;
    const getY = (v) => height - ((v - minVal) / range) * height;

    // Line
    ctx.beginPath();
    ctx.moveTo(getX(0), getY(data[0]));
    for (let i = 1; i < data.length; i++) ctx.lineTo(getX(i), getY(data[i]));

    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    ctx.lineWidth = 2;
    ctx.strokeStyle = currentColor;
    ctx.stroke();

    // Area fill (simple gradient using alpha suffix for HEX colors).
    ctx.lineTo(getX(data.length - 1), height);
    ctx.lineTo(getX(0), height);
    ctx.closePath();

    const gradient = ctx.createLinearGradient(0, 0, 0, height);
    gradient.addColorStop(0, `${currentColor}66`); // ~40% alpha
    gradient.addColorStop(1, `${currentColor}00`); // 0% alpha
    ctx.fillStyle = gradient;
    ctx.fill();
  }
}
