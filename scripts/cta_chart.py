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
import matplotlib.dates as mdates
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
EMA9_COLOR = "#FFD700"     # Yellow
EMA21_COLOR = "#FF8C00"    # Orange
EMA55_COLOR = "#00BCD4"    # Cyan
BB_COLOR = "#555555"
BB_FILL = "#333333"
ST_UP_COLOR = "#26a69a"
ST_DOWN_COLOR = "#ef5350"
RSI_COLOR = "#AB47BC"      # Purple
MACD_COLOR = "#42A5F5"     # Blue
SIGNAL_COLOR = "#FF8C00"   # Orange
FIB_COLOR = "#FFD54F"      # Amber
BULL_ARROW = "#26a69a"
BEAR_ARROW = "#ef5350"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def safe_array(indicators: dict, key: str, length: int):
    """Return indicator array padded/trimmed to `length`, or None if missing."""
    arr = indicators.get(key)
    if arr is None or len(arr) == 0:
        return None
    arr = [float(v) if v is not None else np.nan for v in arr]
    if len(arr) < length:
        arr = [np.nan] * (length - len(arr)) + arr
    elif len(arr) > length:
        arr = arr[-length:]
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
    bars = bars[-bar_limit:]  # Take the last N bars

    # Build DataFrame
    rows = []
    for b in bars:
        dt = b.get("date", "")
        if "T" in dt:
            ts = pd.Timestamp(dt)
        else:
            ts = pd.Timestamp(dt)
        rows.append({
            "Date": ts,
            "Open": float(b["open"]),
            "High": float(b["high"]),
            "Low": float(b["low"]),
            "Close": float(b["close"]),
            "Volume": float(b.get("volume", 0)),
        })

    df = pd.DataFrame(rows)
    df["Date"] = pd.to_datetime(df["Date"])
    df.set_index("Date", inplace=True)
    df.sort_index(inplace=True)

    n = len(df)
    last_date = df.index[-1].strftime("%d %b %Y")
    title = f"{symbol} — {timeframe} — {last_date}"

    # -----------------------------------------------------------------------
    # Build additional plots (addplot)
    # -----------------------------------------------------------------------
    addplots = []

    # EMA lines on main panel
    for key, color, label in [
        ("ema9", EMA9_COLOR, "EMA9"),
        ("ema21", EMA21_COLOR, "EMA21"),
        ("ema55", EMA55_COLOR, "EMA55"),
    ]:
        arr = safe_array(indicators, key, n)
        if arr is not None:
            addplots.append(mpf.make_addplot(
                arr, panel=0, color=color, width=1.0, secondary_y=False
            ))

    # Bollinger Bands on main panel
    bb_upper = safe_array(indicators, "bb_upper", n)
    bb_lower = safe_array(indicators, "bb_lower", n)
    if bb_upper is not None and bb_lower is not None:
        addplots.append(mpf.make_addplot(
            bb_upper, panel=0, color=BB_COLOR, width=0.7, linestyle="--", secondary_y=False
        ))
        addplots.append(mpf.make_addplot(
            bb_lower, panel=0, color=BB_COLOR, width=0.7, linestyle="--", secondary_y=False
        ))
        # Fill between BB bands — we'll do this manually after plotting

    # SuperTrend on main panel (colored segments)
    st_vals = safe_array(indicators, "supertrend", n)
    st_dirs = indicators.get("supertrend_dir", [])
    if st_vals is not None and len(st_dirs) > 0:
        # Pad/trim directions
        if len(st_dirs) < n:
            st_dirs = [""] * (n - len(st_dirs)) + st_dirs
        elif len(st_dirs) > n:
            st_dirs = st_dirs[-n:]

        st_up = [v if d == "UP" else np.nan for v, d in zip(st_vals, st_dirs)]
        st_down = [v if d == "DOWN" else np.nan for v, d in zip(st_vals, st_dirs)]

        addplots.append(mpf.make_addplot(
            st_up, panel=0, color=ST_UP_COLOR, width=1.5, secondary_y=False
        ))
        addplots.append(mpf.make_addplot(
            st_down, panel=0, color=ST_DOWN_COLOR, width=1.5, secondary_y=False
        ))

    # RSI on panel 1
    rsi = safe_array(indicators, "rsi", n)
    if rsi is not None:
        addplots.append(mpf.make_addplot(
            rsi, panel=1, color=RSI_COLOR, width=1.2, ylabel="RSI"
        ))

    # MACD on panel 2
    macd = safe_array(indicators, "macd", n)
    macd_signal = safe_array(indicators, "macd_signal", n)
    macd_hist = safe_array(indicators, "macd_histogram", n)

    if macd is not None:
        addplots.append(mpf.make_addplot(
            macd, panel=2, color=MACD_COLOR, width=1.0, ylabel="MACD"
        ))
    if macd_signal is not None:
        addplots.append(mpf.make_addplot(
            macd_signal, panel=2, color=SIGNAL_COLOR, width=1.0
        ))
    if macd_hist is not None:
        hist_colors = [UP_COLOR if (v or 0) >= 0 else DOWN_COLOR for v in macd_hist]
        addplots.append(mpf.make_addplot(
            macd_hist, panel=2, type="bar", color=hist_colors, width=0.7
        ))

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
    # Panel ratios
    # -----------------------------------------------------------------------
    # Main, RSI, MACD, Volume
    panel_ratios = [5.5, 1.5, 1.5, 1.5]
    num_panels = 4

    # Only include RSI panel if we have RSI data
    has_rsi = rsi is not None
    has_macd = macd is not None
    has_volume = df["Volume"].sum() > 0

    # -----------------------------------------------------------------------
    # Plot
    # -----------------------------------------------------------------------
    fig, axes = mpf.plot(
        df,
        type="candle",
        style=style,
        addplot=addplots if addplots else None,
        volume=has_volume,
        panel_ratios=panel_ratios,
        figsize=(12, 9),
        returnfig=True,
        tight_layout=False,
        warn_too_much_data=999,
    )

    # Title
    fig.suptitle(title, color=TEXT_COLOR, fontsize=12, fontweight="bold", y=0.98)

    # -----------------------------------------------------------------------
    # Post-processing: RSI horizontal lines, Fibonacci levels, pattern arrows
    # -----------------------------------------------------------------------

    # Find the RSI axes (panel 1)
    if has_rsi:
        for ax in axes:
            if hasattr(ax, "get_ylabel") and ax.get_ylabel() == "RSI":
                ax.axhline(70, color=DOWN_COLOR, linestyle="--", linewidth=0.6, alpha=0.7)
                ax.axhline(30, color=UP_COLOR, linestyle="--", linewidth=0.6, alpha=0.7)
                ax.axhline(50, color=GRID_COLOR, linestyle="-", linewidth=0.4, alpha=0.5)
                ax.set_ylim(0, 100)
                break

    # Fibonacci levels on main panel (axes[0])
    main_ax = axes[0]
    fib_levels = fibonacci.get("levels", {})
    if fib_levels:
        for level_name, price_val in fib_levels.items():
            try:
                pv = float(price_val)
                main_ax.axhline(
                    pv, color=FIB_COLOR, linestyle="--", linewidth=0.6, alpha=0.6
                )
                main_ax.text(
                    0.01, pv, f"Fib {level_name}%",
                    transform=main_ax.get_yaxis_transform(),
                    color=FIB_COLOR, fontsize=6, alpha=0.8,
                    verticalalignment="bottom"
                )
            except (ValueError, TypeError):
                pass

    # Pattern annotations on main panel
    if patterns:
        for p in patterns:
            try:
                bar_idx = int(p.get("bar_index", 0))
                direction = p.get("direction", "NEUTRAL")
                name = p.get("name", "")

                # bar_index is in the original (newest-first) data
                # We need to map to DataFrame index (oldest-first)
                plot_idx = n - 1 - bar_idx
                if plot_idx < 0 or plot_idx >= n:
                    continue

                if direction == "BULLISH":
                    marker = "^"
                    color = BULL_ARROW
                    y_val = df.iloc[plot_idx]["Low"] * 0.999
                elif direction == "BEARISH":
                    marker = "v"
                    color = BEAR_ARROW
                    y_val = df.iloc[plot_idx]["High"] * 1.001
                else:
                    continue

                main_ax.plot(
                    plot_idx, y_val, marker=marker, color=color,
                    markersize=8, zorder=10
                )
                # Small label
                main_ax.annotate(
                    name[:15], xy=(plot_idx, y_val),
                    fontsize=5, color=color, alpha=0.8,
                    textcoords="offset points",
                    xytext=(5, 8 if direction == "BULLISH" else -12),
                )
            except (ValueError, TypeError, IndexError):
                pass

    # Bollinger Band fill on main panel
    if bb_upper is not None and bb_lower is not None:
        x_range = range(n)
        main_ax.fill_between(
            x_range,
            [v if not np.isnan(v) else 0 for v in bb_upper],
            [v if not np.isnan(v) else 0 for v in bb_lower],
            alpha=0.08, color=BB_COLOR
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
