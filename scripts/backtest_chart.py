#!/usr/bin/env python3
"""
Backtest Chart Renderer — Equity curve + drawdown chart.

Usage: python3 backtest_chart.py <input.json> <output.png>

Input JSON schema:
  equity_curve: [10000, 10200, ...]
  trade_dates: ["2026-01-05", ...]
  trade_pnl: [2.0, -1.0, ...]
  drawdown: [0, -1.5, -2.1, ...]
  symbol: "EUR/USD"
  timeframe: "Daily"
  params: { start_equity, total_trades, win_rate, total_return, max_dd, sharpe, pf }

Output: 2-panel PNG (1200x600), dark professional theme.
"""

import json
import sys
import warnings

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker
import numpy as np

warnings.filterwarnings("ignore")

# ---------------------------------------------------------------------------
# Color Palette
# ---------------------------------------------------------------------------
BG_COLOR = "#0e1117"
PANEL_BG = "#131720"
GRID_COLOR = "#1e2530"
TEXT_COLOR = "#c9d1d9"
EQUITY_COLOR = "#26a69a"
DD_COLOR = "#ef5350"
DD_FILL = "#ef535040"
WIN_COLOR = "#26a69a"
LOSS_COLOR = "#ef5350"
ZERO_LINE = "#555555"


def render_chart(data: dict, output_path: str):
    equity_curve = data.get("equity_curve", [])
    trade_dates = data.get("trade_dates", [])
    trade_pnl = data.get("trade_pnl", [])
    drawdown = data.get("drawdown", [])
    symbol = data.get("symbol", "???")
    timeframe = data.get("timeframe", "Daily")
    params = data.get("params", {})

    if not equity_curve or len(equity_curve) < 2:
        # Create minimal "no trades" chart
        fig, ax = plt.subplots(figsize=(12, 6), facecolor=BG_COLOR)
        ax.set_facecolor(PANEL_BG)
        ax.text(0.5, 0.5, f"No trades generated for {symbol} ({timeframe})",
                transform=ax.transAxes, ha="center", va="center",
                color=TEXT_COLOR, fontsize=14)
        ax.set_xticks([])
        ax.set_yticks([])
        fig.savefig(output_path, dpi=100, bbox_inches="tight", facecolor=BG_COLOR)
        plt.close(fig)
        return

    n = len(equity_curve)
    x = list(range(n))

    # Compute drawdown from equity if not provided
    if not drawdown or len(drawdown) != n:
        drawdown = []
        peak = equity_curve[0]
        for eq in equity_curve:
            if eq > peak:
                peak = eq
            dd = (eq - peak) / peak * 100.0 if peak > 0 else 0
            drawdown.append(dd)

    # -----------------------------------------------------------------------
    # Figure: 2 panels stacked vertically
    # -----------------------------------------------------------------------
    fig, (ax_eq, ax_dd) = plt.subplots(
        2, 1, figsize=(12, 6),
        gridspec_kw={"height_ratios": [3, 1], "hspace": 0.08},
        facecolor=BG_COLOR,
    )

    # -----------------------------------------------------------------------
    # Panel 1: Equity curve
    # -----------------------------------------------------------------------
    ax_eq.set_facecolor(PANEL_BG)
    ax_eq.plot(x, equity_curve, color=EQUITY_COLOR, linewidth=1.5, zorder=5)
    ax_eq.fill_between(x, equity_curve, equity_curve[0], alpha=0.08, color=EQUITY_COLOR)

    # Trade markers
    if trade_pnl and len(trade_pnl) == n:
        for i, pnl in enumerate(trade_pnl):
            if pnl is None:
                continue
            if pnl > 0:
                ax_eq.plot(i, equity_curve[i], "^", color=WIN_COLOR, markersize=6, zorder=10)
            elif pnl < 0:
                ax_eq.plot(i, equity_curve[i], "v", color=LOSS_COLOR, markersize=6, zorder=10)

    # Grid & labels
    ax_eq.grid(True, color=GRID_COLOR, linestyle=":", alpha=0.5)
    ax_eq.set_ylabel("Equity ($)", color=TEXT_COLOR, fontsize=9)
    ax_eq.tick_params(colors=TEXT_COLOR, labelsize=7)
    ax_eq.set_xlim(0, n - 1)
    ax_eq.yaxis.set_major_formatter(ticker.FuncFormatter(lambda v, _: f"${v:,.0f}"))

    # Hide x-axis labels for top panel (shared with bottom)
    ax_eq.set_xticklabels([])

    # Start equity reference line
    ax_eq.axhline(equity_curve[0], color=ZERO_LINE, linestyle="--", linewidth=0.5, alpha=0.5)

    # -----------------------------------------------------------------------
    # Panel 2: Drawdown
    # -----------------------------------------------------------------------
    ax_dd.set_facecolor(PANEL_BG)
    ax_dd.fill_between(x, drawdown, 0, color=DD_FILL, zorder=5)
    ax_dd.plot(x, drawdown, color=DD_COLOR, linewidth=1.0, zorder=6)
    ax_dd.axhline(0, color=ZERO_LINE, linewidth=0.5)

    ax_dd.grid(True, color=GRID_COLOR, linestyle=":", alpha=0.5)
    ax_dd.set_ylabel("Drawdown %", color=TEXT_COLOR, fontsize=9)
    ax_dd.set_xlabel("Trade #", color=TEXT_COLOR, fontsize=9)
    ax_dd.tick_params(colors=TEXT_COLOR, labelsize=7)
    ax_dd.set_xlim(0, n - 1)
    ax_dd.yaxis.set_major_formatter(ticker.FuncFormatter(lambda v, _: f"{v:.1f}%"))

    # -----------------------------------------------------------------------
    # Title & stats annotation
    # -----------------------------------------------------------------------
    start_eq = params.get("start_equity", equity_curve[0])
    total_trades = params.get("total_trades", n)
    win_rate = params.get("win_rate", 0)
    total_return = params.get("total_return", 0)
    max_dd = params.get("max_dd", min(drawdown) if drawdown else 0)
    sharpe = params.get("sharpe", 0)
    pf = params.get("pf", 0)

    title = f"CTA Backtest: {symbol} ({timeframe})"
    fig.suptitle(title, color=TEXT_COLOR, fontsize=13, fontweight="bold", y=0.98)

    stats_text = (
        f"Trades: {total_trades} | "
        f"WR: {win_rate:.1f}% | "
        f"Return: {total_return:+.1f}% | "
        f"MaxDD: {max_dd:.1f}% | "
        f"Sharpe: {sharpe:.2f} | "
        f"PF: {pf:.2f}x"
    )
    fig.text(0.5, 0.93, stats_text, ha="center", color=TEXT_COLOR, fontsize=8, alpha=0.8)

    # -----------------------------------------------------------------------
    # Save
    # -----------------------------------------------------------------------
    fig.savefig(output_path, dpi=100, bbox_inches="tight", facecolor=BG_COLOR)
    plt.close(fig)
    print(f"Backtest chart saved: {output_path}")


def main():
    if len(sys.argv) < 3:
        print("Usage: python3 backtest_chart.py <input.json> <output.png>", file=sys.stderr)
        sys.exit(1)

    input_path = sys.argv[1]
    output_path = sys.argv[2]

    with open(input_path, "r") as f:
        data = json.load(f)

    render_chart(data, output_path)


if __name__ == "__main__":
    main()
