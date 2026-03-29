#!/usr/bin/env python3
"""
CTA Chart Renderer — Professional TA chart using mplfinance + matplotlib.

Usage: python3 cta_chart.py <input.json> <output.png>

Input JSON schema:
  symbol, timeframe, bars[], indicators{}, fibonacci{}, patterns[]

Output: PNG image (1200x900), dark professional theme.
"""

import json
import sys
import warnings
from datetime import datetime

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import mplfinance as mpf
import numpy as np
import pandas as pd

warnings.filterwarnings("ignore")

# ---------------------------------------------------------------------------
# Color Palette (Professional Dark Theme)
# ---------------------------------------------------------------------------
BG_COLOR = "#0e1117"
PANEL_BG = "#0e1117"
GRID_COLOR = "#1e2530"
TEXT_COLOR = "#c9d1d9"
UP_COLOR = "#26a69a"
DOWN_COLOR = "#ef5350"
EMA9_COLOR = "#FFD700"
EMA21_COLOR = "#FF8C00"
EMA55_COLOR = "#00BCD4"
BB_COLOR = "#555555"
ST_UP_COLOR = "#26a69a"
ST_DOWN_COLOR = "#ef5350"
RSI_COLOR = "#AB47BC"
MACD_COLOR = "#42A5F5"
SIGNAL_COLOR = "#FF8C00"
FIB_COLOR = "#FFD54F"
BULL_ARROW = "#26a69a"
BEAR_ARROW = "#ef5350"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def safe_array(indicators: dict, key: str, length: int, price_range=None):
    """Return indicator array padded/trimmed to `length`, or None if missing.
    If price_range=(lo, hi) is given, clip values outside that range to NaN."""
    arr = indicators.get(key)
    if arr is None or len(arr) == 0:
        return None
    arr = [float(v) if v is not None else np.nan for v in arr]
    if len(arr) < length:
        arr = [np.nan] * (length - len(arr)) + arr
    elif len(arr) > length:
        arr = arr[-length:]
    # Clip extreme values (0, or wildly outside price range) to NaN
    if price_range is not None:
        lo, hi = price_range
        margin = (hi - lo) * 2  # allow 2x range
        arr = [v if (not np.isnan(v) and lo - margin < v < hi + margin) else np.nan for v in arr]
    return arr


def determine_bar_limit(timeframe: str) -> int:
    tf = timeframe.lower()
    if tf in ("daily", "d", "1d"):
        return 80
    elif tf in ("weekly", "w", "1w"):
        return 52
    else:
        return 96


def render_chart(data: dict, output_path: str):
    bars = data.get("bars", [])
    if not bars:
        print("ERROR: No bars in input data", file=sys.stderr)
        sys.exit(1)

    symbol = data.get("symbol", "???")
    timeframe = data.get("timeframe", "")
    indicators = data.get("indicators", {})
    fibonacci = data.get("fibonacci", {})
    patterns = data.get("patterns", [])

    # Determine how many bars to show
    bar_limit = determine_bar_limit(timeframe)
    bars = bars[-bar_limit:]

    # Build DataFrame
    rows = []
    for b in bars:
        dt = b.get("date", "")
        ts = pd.Timestamp(dt)
        o, h, l, c = float(b["open"]), float(b["high"]), float(b["low"]), float(b["close"])
        v = float(b.get("volume", 0))
        # Skip bars with 0 prices (bad data)
        if o <= 0 or h <= 0 or l <= 0 or c <= 0:
            continue
        rows.append({"Date": ts, "Open": o, "High": h, "Low": l, "Close": c, "Volume": v})

    if not rows:
        print("ERROR: No valid bars after filtering", file=sys.stderr)
        sys.exit(1)

    df = pd.DataFrame(rows)
    df["Date"] = pd.to_datetime(df["Date"])
    df.set_index("Date", inplace=True)
    df.sort_index(inplace=True)

    n = len(df)
    last_date = df.index[-1].strftime("%d %b %Y")
    title = f"{symbol} — {timeframe} — {last_date}"

    # Price range for clipping indicator values
    price_lo = df["Low"].min()
    price_hi = df["High"].max()
    price_range = (price_lo, price_hi)

    # -----------------------------------------------------------------------
    # Determine which panels exist
    # Panel 0: main candlestick (always)
    # Panel 1: RSI (if available)
    # Panel 2: MACD (if available)
    # Volume is overlaid on main panel (panel 0) to avoid panel count issues
    # -----------------------------------------------------------------------
    rsi = safe_array(indicators, "rsi", n)
    macd = safe_array(indicators, "macd", n)
    macd_signal = safe_array(indicators, "macd_signal", n)
    macd_hist = safe_array(indicators, "macd_histogram", n)

    has_rsi = rsi is not None
    has_macd = macd is not None
    has_volume = bool(df["Volume"].sum() > 0)

    # Assign panel numbers dynamically
    rsi_panel = None
    macd_panel = None
    next_panel = 1

    if has_rsi:
        rsi_panel = next_panel
        next_panel += 1
    if has_macd:
        macd_panel = next_panel
        next_panel += 1

    # Build panel_ratios: one entry per panel
    panel_ratios = [5.5]  # panel 0
    if has_rsi:
        panel_ratios.append(1.5)
    if has_macd:
        panel_ratios.append(1.5)

    # -----------------------------------------------------------------------
    # Build addplots
    # -----------------------------------------------------------------------
    addplots = []

    # EMA lines on main panel
    for key, color in [("ema9", EMA9_COLOR), ("ema21", EMA21_COLOR), ("ema55", EMA55_COLOR)]:
        arr = safe_array(indicators, key, n, price_range)
        if arr is not None:
            addplots.append(mpf.make_addplot(arr, panel=0, color=color, width=1.0, secondary_y=False))

    # Bollinger Bands on main panel
    bb_upper = safe_array(indicators, "bb_upper", n, price_range)
    bb_lower = safe_array(indicators, "bb_lower", n, price_range)
    if bb_upper is not None and bb_lower is not None:
        addplots.append(mpf.make_addplot(bb_upper, panel=0, color=BB_COLOR, width=0.7, linestyle="--", secondary_y=False))
        addplots.append(mpf.make_addplot(bb_lower, panel=0, color=BB_COLOR, width=0.7, linestyle="--", secondary_y=False))

    # SuperTrend on main panel (colored segments)
    st_vals = safe_array(indicators, "supertrend", n, price_range)
    st_dirs = indicators.get("supertrend_dir", [])
    if st_vals is not None and len(st_dirs) > 0:
        if len(st_dirs) < n:
            st_dirs = [""] * (n - len(st_dirs)) + st_dirs
        elif len(st_dirs) > n:
            st_dirs = st_dirs[-n:]
        st_up = [v if d == "UP" else np.nan for v, d in zip(st_vals, st_dirs)]
        st_down = [v if d == "DOWN" else np.nan for v, d in zip(st_vals, st_dirs)]
        addplots.append(mpf.make_addplot(st_up, panel=0, color=ST_UP_COLOR, width=1.5, secondary_y=False))
        addplots.append(mpf.make_addplot(st_down, panel=0, color=ST_DOWN_COLOR, width=1.5, secondary_y=False))

    # RSI
    if has_rsi:
        addplots.append(mpf.make_addplot(rsi, panel=rsi_panel, color=RSI_COLOR, width=1.2, ylabel="RSI"))

    # MACD
    if has_macd:
        addplots.append(mpf.make_addplot(macd, panel=macd_panel, color=MACD_COLOR, width=1.0, ylabel="MACD"))
    if macd_signal is not None and macd_panel is not None:
        addplots.append(mpf.make_addplot(macd_signal, panel=macd_panel, color=SIGNAL_COLOR, width=1.0))
    if macd_hist is not None and macd_panel is not None:
        hist_colors = [UP_COLOR if (v or 0) >= 0 else DOWN_COLOR for v in macd_hist]
        addplots.append(mpf.make_addplot(macd_hist, panel=macd_panel, type="bar", color=hist_colors, width=0.7))

    # -----------------------------------------------------------------------
    # Custom style
    # -----------------------------------------------------------------------
    mc = mpf.make_marketcolors(
        up=UP_COLOR, down=DOWN_COLOR,
        edge={"up": UP_COLOR, "down": DOWN_COLOR},
        wick={"up": UP_COLOR, "down": DOWN_COLOR},
        volume={"up": UP_COLOR, "down": DOWN_COLOR},
    )
    style = mpf.make_mpf_style(
        marketcolors=mc,
        figcolor=BG_COLOR,
        facecolor=PANEL_BG,
        gridcolor=GRID_COLOR,
        gridstyle=":",
        gridaxis="both",
        rc={
            "axes.labelcolor": TEXT_COLOR,
            "xtick.color": TEXT_COLOR,
            "ytick.color": TEXT_COLOR,
            "font.size": 8,
        },
    )

    # -----------------------------------------------------------------------
    # Plot — volume=False to avoid extra panel, we skip volume for simplicity
    # -----------------------------------------------------------------------
    plot_kwargs = dict(
        type="candle",
        style=style,
        volume=False,  # Disable auto-volume panel to keep panel count clean
        panel_ratios=panel_ratios if len(panel_ratios) > 1 else None,
        figsize=(14, 9),
        returnfig=True,
        tight_layout=False,
        warn_too_much_data=999,
    )
    if addplots:
        plot_kwargs["addplot"] = addplots
    if plot_kwargs["panel_ratios"] is None:
        del plot_kwargs["panel_ratios"]

    fig, axes = mpf.plot(df, **plot_kwargs)

    # Title
    fig.suptitle(title, color=TEXT_COLOR, fontsize=13, fontweight="bold", y=0.98)

    # -----------------------------------------------------------------------
    # Post-processing
    # -----------------------------------------------------------------------
    main_ax = axes[0]

    # RSI horizontal lines
    if has_rsi:
        for ax in axes:
            if hasattr(ax, "get_ylabel") and ax.get_ylabel() == "RSI":
                ax.axhline(70, color=DOWN_COLOR, linestyle="--", linewidth=0.6, alpha=0.7)
                ax.axhline(30, color=UP_COLOR, linestyle="--", linewidth=0.6, alpha=0.7)
                ax.axhline(50, color=GRID_COLOR, linestyle="-", linewidth=0.4, alpha=0.5)
                ax.set_ylim(0, 100)
                break

    # Fibonacci levels
    fib_levels = fibonacci.get("levels", {})
    if fib_levels:
        for level_name, price_val in fib_levels.items():
            try:
                pv = float(price_val)
                if price_lo * 0.9 < pv < price_hi * 1.1:  # Only draw if in visible range
                    main_ax.axhline(pv, color=FIB_COLOR, linestyle="--", linewidth=0.6, alpha=0.5)
                    main_ax.text(
                        0.01, pv, f"Fib {level_name}%",
                        transform=main_ax.get_yaxis_transform(),
                        color=FIB_COLOR, fontsize=6, alpha=0.7,
                        verticalalignment="bottom"
                    )
            except (ValueError, TypeError):
                pass

    # Pattern annotations
    if patterns:
        for p in patterns[-10:]:  # Max 10 patterns to avoid clutter
            try:
                bar_idx = int(p.get("bar_index", 0))
                direction = p.get("direction", "NEUTRAL")
                name = p.get("name", "")
                plot_idx = n - 1 - bar_idx
                if plot_idx < 0 or plot_idx >= n:
                    continue

                if direction == "BULLISH":
                    marker, color = "^", BULL_ARROW
                    y_val = df.iloc[plot_idx]["Low"] * 0.999
                elif direction == "BEARISH":
                    marker, color = "v", BEAR_ARROW
                    y_val = df.iloc[plot_idx]["High"] * 1.001
                else:
                    continue

                main_ax.plot(plot_idx, y_val, marker=marker, color=color, markersize=8, zorder=10)
                main_ax.annotate(
                    name[:15], xy=(plot_idx, y_val),
                    fontsize=5, color=color, alpha=0.8,
                    textcoords="offset points",
                    xytext=(5, 8 if direction == "BULLISH" else -12),
                )
            except (ValueError, TypeError, IndexError):
                pass

    # BB fill
    if bb_upper is not None and bb_lower is not None:
        x_range = range(n)
        main_ax.fill_between(
            x_range,
            [v if not np.isnan(v) else 0 for v in bb_upper],
            [v if not np.isnan(v) else 0 for v in bb_lower],
            alpha=0.06, color=BB_COLOR
        )

    # -----------------------------------------------------------------------
    # Save
    # -----------------------------------------------------------------------
    fig.savefig(output_path, dpi=100, bbox_inches="tight", facecolor=BG_COLOR)
    plt.close(fig)
    print(f"Chart saved: {output_path}")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    if len(sys.argv) < 3:
        print("Usage: python3 cta_chart.py <input.json> <output.png>", file=sys.stderr)
        sys.exit(1)

    input_path = sys.argv[1]
    output_path = sys.argv[2]

    with open(input_path, "r") as f:
        data = json.load(f)

    render_chart(data, output_path)


if __name__ == "__main__":
    main()
